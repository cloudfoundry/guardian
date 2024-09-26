//go:build !windows

package dadoo

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"syscall"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/signals"
	"code.cloudfoundry.org/lager/v3"
)

type ExecRunner struct {
	dadooPath                string
	runcPath                 string
	runcRoot                 string
	signallerFactory         *signals.SignallerFactory
	commandRunner            commandrunner.CommandRunner
	cleanupProcessDirsOnWait bool
	processes                map[string]*process
	processesMutex           *sync.Mutex
	containerRootHostUID     uint32
	containerRootHostGID     uint32
	bundleSaver              depot.BundleSaver
	bundleLookupper          depot.BundleLookupper
	processDepot             execrunner.ProcessDepot
}

func NewExecRunner(
	dadooPath, runcPath, runcRoot string, signallerFactory *signals.SignallerFactory,
	commandRunner commandrunner.CommandRunner, shouldCleanup bool, containerRootHostUID, containerRootHostGID uint32,
	bundleSaver depot.BundleSaver, bundleLookupper depot.BundleLookupper,
	processDepot execrunner.ProcessDepot,
) *ExecRunner {
	return &ExecRunner{
		dadooPath:                dadooPath,
		runcPath:                 runcPath,
		runcRoot:                 runcRoot,
		signallerFactory:         signallerFactory,
		commandRunner:            commandRunner,
		cleanupProcessDirsOnWait: shouldCleanup,
		processes:                map[string]*process{},
		processesMutex:           new(sync.Mutex),
		containerRootHostUID:     containerRootHostUID,
		containerRootHostGID:     containerRootHostGID,
		bundleSaver:              bundleSaver,
		bundleLookupper:          bundleLookupper,
		processDepot:             processDepot,
	}
}

func (d *ExecRunner) Run(
	log lager.Logger, processID, sandboxHandle string,
	pio garden.ProcessIO, tty bool, procJSON io.Reader, extraCleanup func() error,
) (proc garden.Process, theErr error) {
	log = log.Session("execrunner", lager.Data{"id": processID})

	log.Info("start")
	defer log.Info("done")

	processPath, err := d.processDepot.CreateProcessDir(log, sandboxHandle, processID)
	if err != nil {
		return nil, err
	}

	defer func() {
		if theErr == nil {
			return
		}

		if err := os.RemoveAll(processPath); err != nil {
			log.Info("failed-to-remove-process-dir: "+err.Error(), lager.Data{"process_id": processID})
		}
	}()

	return d.runProcess(log, "exec", processID, processPath, sandboxHandle, pio, tty, procJSON, extraCleanup)
}

func (d *ExecRunner) RunPea(
	log lager.Logger, processID string, processBundle goci.Bndl, sandboxHandle string,
	pio garden.ProcessIO, tty bool, procJSON io.Reader, extraCleanup func() error,
) (proc garden.Process, theErr error) {
	log = log.Session("execrunner", lager.Data{"id": processID})

	log.Info("start")
	defer log.Info("done")

	processPath, err := d.processDepot.CreateProcessDir(log, sandboxHandle, processID)
	if err != nil {
		return nil, err
	}

	err = d.bundleSaver.Save(processBundle, processPath)
	if err != nil {
		return nil, err
	}

	defer func() {
		if theErr == nil {
			return
		}

		if err := os.RemoveAll(processPath); err != nil {
			log.Info("failed-to-remove-process-dir: "+err.Error(), lager.Data{"process_id": processID})
		}
	}()

	return d.runProcess(log, "run", processID, processPath, sandboxHandle, pio, tty, procJSON, extraCleanup)
}

func (d *ExecRunner) runProcess(
	log lager.Logger, runMode, processID, processPath, sandboxHandle string,
	pio garden.ProcessIO, tty bool, procJSON io.Reader, extraCleanup func() error,
) (proc garden.Process, theErr error) {
	log.Info("run-process")
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

	process := d.getProcess(log, processID, processPath, filepath.Join(processPath, "pidfile"), extraCleanup)
	if err := process.mkfifos(d.containerRootHostUID, d.containerRootHostGID); err != nil {
		return nil, err
	}

	cmd := buildDadooCommand(
		tty, d.dadooPath, runMode, d.runcPath, d.runcRoot, processID, processPath, sandboxHandle,
		[]*os.File{fd3w, logw, syncw}, procJSON,
	)

	// TODO this is kind of an hack - do we have a better place for dadoo logs
	// that would not depend on the dir structure of the depot?
	dadooLogFilePath := filepath.Join(processPath, "..", "..", fmt.Sprintf("dadoo.%s.log", processID))
	dadooLogFile, err := os.Create(dadooLogFilePath)
	if err != nil {
		return nil, err
	}
	defer dadooLogFile.Close()
	cmd.Stdout = dadooLogFile
	cmd.Stderr = dadooLogFile

	log.Info("dadoo-start-command", lager.Data{"path": d.dadooPath})
	if err := d.commandRunner.Start(cmd); err != nil {
		log.Info("failed-to-start")
		return nil, err
	}
	go func() {
		// wait on spawned process to avoid zombies
		waitErr := d.commandRunner.Wait(cmd)
		if waitErr != nil {
			log.Debug("faild-waiting-on-process", lager.Data{"error": waitErr})
		}
		if copyErr := copyDadooLogsToGuardianLogger(dadooLogFilePath, log); copyErr != nil {
			log.Error("reading-dadoo-log-file", copyErr)
		}
	}()
	log.Info("dadoo-close")

	err = fd3w.Close()
	if err != nil {
		log.Debug("failed-closing-fd3-writer", lager.Data{"error": err})
	}
	err = logw.Close()
	if err != nil {
		log.Debug("failed-closing-log-writer", lager.Data{"error": err})
	}
	err = syncw.Close()
	if err != nil {
		log.Debug("failed-closing-sync-writer", lager.Data{"error": err})
	}

	log.Info("dadoo-open-files")

	stdin, stdout, stderr, err := process.openPipes(pio)
	if err != nil {
		return nil, err
	}

	log.Info("dadoo-syncr-read")
	syncMsg := make([]byte, 1)
	_, err = syncr.Read(syncMsg)
	if err != nil {
		return nil, err
	}

	log.Info("dadoo-stream-data")
	process.streamData(pio, stdin, stdout, stderr)

	doneReadingRuncLogs := make(chan string)
	go func(log lager.Logger, logs io.Reader, logTag string, done chan<- string) {
		scanner := bufio.NewScanner(logs)
		lastError := ""
		for scanner.Scan() {
			nextLogLine := scanner.Bytes()
			logLine := logging.ForwardRuncLogsToLager(log, logTag, nextLogLine)
			if logLine.IsError() {
				lastError = logLine.Msg
			}
		}
		done <- lastError
	}(log, logr, "runc", doneReadingRuncLogs)

	defer func() {
		lastErrorMessage := <-doneReadingRuncLogs
		if theErr == nil {
			return
		}

		if isNoSuchExecutable(lastErrorMessage) {
			theErr = garden.ExecutableNotFoundError{Message: lastErrorMessage}
			return
		}

		theErr = logging.WrapWithErrorFromLastMessage("runc exec", theErr, lastErrorMessage)
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

func isNoSuchExecutable(logLine string) bool {
	runcError := `(?:starting container process caused|unable to start container process)`
	noSuchFile := `stat .*: no such file or directory`
	executableNotFound := `executable file not found in \$PATH`
	noSuchExecutable := regexp.MustCompile(fmt.Sprintf(`%s: exec: .*: (?:%s|%s)`, runcError, noSuchFile, executableNotFound))

	return noSuchExecutable.MatchString(logLine)
}

func buildDadooCommand(tty bool, dadooPath, dadooRunMode, runcPath, runcRoot, processID, processPath, sandboxHandle string, extraFiles []*os.File, stdin io.Reader) *exec.Cmd {
	dadooArgs := []string{"-runc-root", runcRoot}
	if tty {
		dadooArgs = append(dadooArgs, "-tty")
	}
	dadooArgs = append(dadooArgs, dadooRunMode, runcPath, processPath)
	if dadooRunMode == "run" {
		dadooArgs = append(dadooArgs, processID)
	} else {
		dadooArgs = append(dadooArgs, sandboxHandle)
	}

	cmd := exec.Command(dadooPath, dadooArgs...)
	cmd.ExtraFiles = extraFiles
	cmd.Stdin = stdin

	return cmd
}

func (d *ExecRunner) Attach(log lager.Logger, sandboxHandle, processID string, io garden.ProcessIO) (garden.Process, error) {
	processPath, err := d.processDepot.LookupProcessDir(log, sandboxHandle, processID)
	if err != nil {
		return nil, garden.ProcessNotFoundError{ProcessID: processID}
	}

	process := d.getProcess(log, processID, processPath, filepath.Join(processPath, "pidfile"), nil)
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
	stdoutWriter                                 *execrunner.DynamicMultiWriter
	stderrWriter                                 *execrunner.DynamicMultiWriter
	streamMutex                                  *sync.Mutex

	signals.Signaller
}

func (d *ExecRunner) getProcess(log lager.Logger, id, processPath, pidFilePath string, extraCleanup func() error) *process {
	d.processesMutex.Lock()
	defer d.processesMutex.Unlock()

	if existingProcess, ok := d.processes[processPath]; ok {
		return existingProcess
	}

	cleanupFunc := func() error {
		d.processesMutex.Lock()
		delete(d.processes, processPath)
		d.processesMutex.Unlock()

		defer func() {
			if d.cleanupProcessDirsOnWait {
				err := os.RemoveAll(processPath)
				if err != nil {
					log.Error("error-cleaning-up-process-dir", err, lager.Data{"processPath": processPath})
				}
			}
		}()

		if extraCleanup != nil {
			if err := extraCleanup(); err != nil {
				return err
			}
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
		stdoutWriter: execrunner.NewDynamicMultiWriter(),
		stderrWriter: execrunner.NewDynamicMultiWriter(),
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
			_, err := io.Copy(stdin, pio.Stdin)
			if err != nil {
				p.logger.Debug("faild-copying-from-stdin", lager.Data{"error": err})
			}
			err = stdin.Close()
			if err != nil {
				p.logger.Debug("faild-closing-stdin", lager.Data{"error": err})
			}
		}()
	}

	if pio.Stdout != nil {
		p.stdoutWriter.Attach(pio.Stdout)
		if p.stdoutWriter.Count() == 1 {
			p.ioWg.Add(1)
			go func() {
				_, err := io.Copy(p.stdoutWriter, stdout)
				if err != nil {
					p.logger.Debug("faild-copying-from-stdout", lager.Data{"error": err})
				}
				err = stdout.Close()
				if err != nil {
					p.logger.Debug("faild-closing-stdout", lager.Data{"error": err})
				}
				p.ioWg.Done()
			}()
		}
	}

	if pio.Stderr != nil {
		p.stderrWriter.Attach(pio.Stderr)
		if p.stderrWriter.Count() == 1 {
			p.ioWg.Add(1)
			go func() {
				_, err := io.Copy(p.stderrWriter, stderr)
				if err != nil {
					p.logger.Debug("faild-copying-from-stderr", lager.Data{"error": err})
				}
				err = stderr.Close()
				if err != nil {
					p.logger.Debug("faild-closing-stderr", lager.Data{"error": err})
				}
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
	_, err = exit.Read(buf)
	if err != nil {
		p.logger.Debug("faild-reading-from-pipe", lager.Data{"error": err})
	}

	p.ioWg.Wait()

	if _, err := os.Stat(p.exitcode); os.IsNotExist(err) {
		return 1, fmt.Errorf("could not find the exitcode file for the process: %s", err.Error())
	}

	exitcode, err := os.ReadFile(p.exitcode)
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

	dadooLogs, err := os.ReadFile(dadooLogFilePath)
	if err != nil {
		return fmt.Errorf("failed to read dadoo log file: %s", err)
	}
	logger.Debug("dadoo", lager.Data{"message": string(dadooLogs)})
	return nil
}
