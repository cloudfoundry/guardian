package dadoo

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/guardian/rundmc/signals"
	"code.cloudfoundry.org/lager"
)

type ExecRunner struct {
	dadooPath                string
	runcPath                 string
	signallerFactory         *signals.SignallerFactory
	commandRunner            commandrunner.CommandRunner
	cleanupProcessDirsOnWait bool
	processes                map[string]*process
	processesMutex           *sync.Mutex
	runMode                  string
}

func NewExecRunner(
	dadooPath, runcPath string, signallerFactory *signals.SignallerFactory,
	commandRunner commandrunner.CommandRunner, shouldCleanup bool, runMode string,
) *ExecRunner {
	return &ExecRunner{
		dadooPath:                dadooPath,
		runcPath:                 runcPath,
		signallerFactory:         signallerFactory,
		commandRunner:            commandRunner,
		cleanupProcessDirsOnWait: shouldCleanup,
		processes:                map[string]*process{},
		processesMutex:           new(sync.Mutex),
		runMode:                  runMode,
	}
}

func (d *ExecRunner) Run(
	log lager.Logger, processID, processPath, sandboxHandle, sandboxBundlePath string,
	containerRootHostUID, containerRootHostGID uint32, pio garden.ProcessIO, tty bool,
	procJSON io.Reader,
) (proc garden.Process, theErr error) {
	log = log.Session("execrunner")

	log.Info("start")
	defer log.Info("done")

	fd3r, fd3w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer fd3r.Close()

	logr, logw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer logr.Close()

	syncr, syncw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer syncr.Close()

	process := d.getProcess(log, processID, processPath, filepath.Join(processPath, "pidfile"))
	if err := process.mkfifos(containerRootHostUID, containerRootHostGID); err != nil {
		return nil, err
	}

	cmd := buildDadooCommand(
		tty, d.dadooPath, d.runMode, d.runcPath, processPath, sandboxHandle,
		[]*os.File{fd3w, logw, syncw}, procJSON,
	)

	dadooLogFilePath := filepath.Join(sandboxBundlePath, fmt.Sprintf("dadoo.%s.log", processID))
	dadooLogFile, err := os.Create(dadooLogFilePath)
	if err != nil {
		return nil, err
	}
	defer dadooLogFile.Close()
	cmd.Stdout = dadooLogFile
	cmd.Stderr = dadooLogFile

	if err := d.commandRunner.Start(cmd); err != nil {
		return nil, err
	}
	go func() {
		// wait on spawned process to avoid zombies
		d.commandRunner.Wait(cmd)
		if copyErr := copyDadooLogsToGuardianLogger(dadooLogFilePath, log); copyErr != nil {
			log.Error("reading-dadoo-log-file", copyErr)
		}
	}()

	fd3w.Close()
	logw.Close()
	syncw.Close()

	stdin, stdout, stderr, err := process.openPipes(pio)
	if err != nil {
		return nil, err
	}

	syncMsg := make([]byte, 1)
	_, err = syncr.Read(syncMsg)
	if err != nil {
		return nil, err
	}

	process.streamData(pio, stdin, stdout, stderr)

	doneReadingRuncLogs := make(chan []byte)
	go func(log lager.Logger, logs io.Reader, logTag string, done chan<- []byte) {
		scanner := bufio.NewScanner(logs)

		nextLogLine := []byte("")
		for scanner.Scan() {
			nextLogLine = scanner.Bytes()
			logging.ForwardRuncLogsToLager(log, logTag, nextLogLine)
		}
		done <- nextLogLine
	}(log, logr, "runc", doneReadingRuncLogs)

	defer func() {
		lastLogLine := <-doneReadingRuncLogs
		if theErr != nil {
			theErr = logging.WrapWithErrorFromLastLogLine("runc exec", theErr, lastLogLine)
		}
	}()

	log.Info("read-exit-fd")
	runcExitStatus := make([]byte, 1)
	bytesRead, err := fd3r.Read(runcExitStatus)
	if bytesRead == 0 || err != nil {
		return nil, fmt.Errorf("failed to read runc exit code %v", err)
	}
	log.Info("runc-exit-status", lager.Data{"status": runcExitStatus[0]})
	if runcExitStatus[0] != 0 {
		return nil, fmt.Errorf("exit status %d", runcExitStatus[0])
	}

	return process, nil
}

func buildDadooCommand(tty bool, dadooPath, dadooRunMode, runcPath, processPath, handle string, extraFiles []*os.File, stdin io.Reader) *exec.Cmd {
	var cmd *exec.Cmd
	if tty {
		cmd = exec.Command(dadooPath, "-tty", dadooRunMode, runcPath, processPath, handle)
	} else {
		cmd = exec.Command(dadooPath, dadooRunMode, runcPath, processPath, handle)
	}
	cmd.ExtraFiles = extraFiles
	cmd.Stdin = stdin

	return cmd
}

func (d *ExecRunner) Attach(log lager.Logger, processID string, io garden.ProcessIO, processesPath string) (garden.Process, error) {
	processPath := filepath.Join(processesPath, processID)
	if _, err := os.Stat(processPath); err != nil {
		if os.IsNotExist(err) {
			return nil, garden.ProcessNotFoundError{ProcessID: processID}
		}
		return nil, err
	}

	process := d.getProcess(log, processID, processPath, filepath.Join(processPath, "pidfile"))
	if err := process.attach(io); err != nil {
		return nil, err
	}

	return process, nil
}

type process struct {
	logger                                       lager.Logger
	id                                           string
	stdin, stdout, stderr, exit, winsz, exitcode string
	ioWg                                         *sync.WaitGroup
	winszCh                                      chan garden.WindowSize
	cleanup                                      func() error
	stdoutWriter                                 *DynamicMultiWriter
	stderrWriter                                 *DynamicMultiWriter
	streamMutex                                  *sync.Mutex

	signals.Signaller
}

func (d *ExecRunner) getProcess(log lager.Logger, id, processPath, pidFilePath string) *process {
	d.processesMutex.Lock()
	defer d.processesMutex.Unlock()

	if existingProcess, ok := d.processes[processPath]; ok {
		return existingProcess
	}

	cleanupFunc := func() error {
		d.processesMutex.Lock()
		delete(d.processes, processPath)
		d.processesMutex.Unlock()
		if d.cleanupProcessDirsOnWait {
			return os.RemoveAll(processPath)
		}
		return nil
	}

	d.processes[processPath] = &process{
		logger:       log,
		id:           id,
		stdin:        filepath.Join(processPath, "stdin"),
		stdout:       filepath.Join(processPath, "stdout"),
		stderr:       filepath.Join(processPath, "stderr"),
		winsz:        filepath.Join(processPath, "winsz"),
		exit:         filepath.Join(processPath, "exit"),
		exitcode:     filepath.Join(processPath, "exitcode"),
		ioWg:         &sync.WaitGroup{},
		winszCh:      make(chan garden.WindowSize, 5),
		cleanup:      cleanupFunc,
		Signaller:    d.signallerFactory.NewSignaller(pidFilePath),
		stdoutWriter: NewDynamicMultiWriter(),
		stderrWriter: NewDynamicMultiWriter(),
		streamMutex:  new(sync.Mutex),
	}
	return d.processes[processPath]
}

func (p *process) ID() string {
	return p.id
}

func (p *process) mkfifos(hostUid, hostGid uint32) error {
	for _, pipe := range []string{p.stdin, p.stdout, p.stderr, p.winsz, p.exit} {
		if err := syscall.Mkfifo(pipe, 0600); err != nil {
			return err
		}
		if err := os.Chown(pipe, int(hostUid), int(hostGid)); err != nil {
			return err
		}
	}

	return nil
}

func (p process) openPipes(pio garden.ProcessIO) (stdin, stdout, stderr *os.File, err error) {
	stdin, err = os.OpenFile(p.stdin, os.O_RDWR, 0600)
	if err != nil {
		return nil, nil, nil, err
	}
	stdout, err = openNonBlocking(p.stdout)
	if err != nil {
		return nil, nil, nil, err
	}
	stderr, err = openNonBlocking(p.stderr)
	if err != nil {
		return nil, nil, nil, err
	}

	return stdin, stdout, stderr, nil
}

func openNonBlocking(fileName string) (*os.File, error) {
	file, err := os.OpenFile(fileName, os.O_RDONLY|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.SetNonblock(int(file.Fd()), false); err != nil {
		return nil, err
	}
	return file, nil
}

func (p process) streamData(pio garden.ProcessIO, stdin, stdout, stderr *os.File) {
	p.streamMutex.Lock()
	defer p.streamMutex.Unlock()

	if pio.Stdin != nil {
		go func() {
			io.Copy(stdin, pio.Stdin)
			stdin.Close()
		}()
	}

	if pio.Stdout != nil {
		p.stdoutWriter.Attach(pio.Stdout)
		if p.stdoutWriter.Count() == 1 {
			p.ioWg.Add(1)
			go func() {
				io.Copy(p.stdoutWriter, stdout)
				stdout.Close()
				p.ioWg.Done()
			}()
		}
	}

	if pio.Stderr != nil {
		p.stderrWriter.Attach(pio.Stderr)
		if p.stderrWriter.Count() == 1 {
			p.ioWg.Add(1)
			go func() {
				io.Copy(p.stderrWriter, stderr)
				stderr.Close()
				p.ioWg.Done()
			}()
		}
	}
}

func (p process) attach(pio garden.ProcessIO) error {
	stdin, stdout, stderr, err := p.openPipes(pio)
	if err != nil {
		return err
	}

	p.streamData(pio, stdin, stdout, stderr)

	return nil
}

func (p process) Wait() (int, error) {
	// open non-blocking incase exit pipe is already closed
	exit, err := openNonBlocking(p.exit)
	if err != nil {
		return 1, err
	}
	defer exit.Close()

	buf := make([]byte, 1)
	exit.Read(buf)

	p.ioWg.Wait()

	if _, err := os.Stat(p.exitcode); os.IsNotExist(err) {
		return 1, fmt.Errorf("could not find the exitcode file for the process: %s", err.Error())
	}

	exitcode, err := ioutil.ReadFile(p.exitcode)
	if err != nil {
		return 1, err
	}

	if len(exitcode) == 0 {
		return 1, fmt.Errorf("the exitcode file is empty")
	}

	code, err := strconv.Atoi(string(exitcode))
	if err != nil {
		return 1, fmt.Errorf("failed to parse exit code: %s", err.Error())
	}

	if err := p.cleanup(); err != nil {
		p.logger.Error("process-cleanup", err)
	}

	return code, nil
}

func (p process) SetTTY(spec garden.TTYSpec) error {
	if spec.WindowSize == nil {
		return nil
	}

	winSize, err := os.OpenFile(p.winsz, os.O_WRONLY|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return err
	}

	defer winSize.Close()
	return json.NewEncoder(winSize).Encode(spec.WindowSize)
}

func copyDadooLogsToGuardianLogger(dadooLogFilePath string, logger lager.Logger) error {
	dadooLogFileInfo, err := os.Stat(dadooLogFilePath)
	if err != nil {
		return fmt.Errorf("stating dadoo log file: %s", err)
	}
	defer func() {
		if err := os.Remove(dadooLogFilePath); err != nil {
			logger.Error("removing-dadoo-log-file", err)
		}
	}()
	if dadooLogFileInfo.Size() == 0 {
		return nil
	}

	dadooLogs, err := ioutil.ReadFile(dadooLogFilePath)
	if err != nil {
		return fmt.Errorf("failed to read dadoo log file: %s", err)
	}
	logger.Debug("dadoo", lager.Data{"message": string(dadooLogs)})
	return nil
}
