# debugserver

**Note**: This repository should be imported as `code.cloudfoundry.org/debugserver`.

A helper function for running a pre-configured
[pprof](http://golang.org/pkg/net/http/pprof/) server in go.

## Endpoints

- `/log-level`

 Sets the log level of the sink passed to the `Runner` method. This endpoint
 expects the request method to be POST or PUT and uses the body of the request as the
 new log level. For example, `curl -X POST --data 'debug' http://host:port/log-level`
 will set the log level to `debug`.

- `/debug/pprof/cmdline`

 Responds with the running program's
 command line, with arguments separated by NUL bytes.

- `/debug/pprof/profile`

 Responds with the pprof-formatted CPU profile.

- `/debug/pprof/heap`

Responds with the pprof-formatted heap profile.

- `/debug/pprof/block`

Responds with the pprof-formatted goroutine blocking profile.

- `/debug/pprof/trace?seconds=n`

Responds with the pprof-formatted execution trace for n seconds.

- `/debug/pprof/symbol`

 Looks up the program counters listed in the request,
 responding with a table mapping program counters to function names.

- `/block-profile-rate`

 Controls the fraction of goroutine blocking events
 that are reported in the blocking profile. The profiler aims to sample
 an average of one blocking event per rate nanoseconds spent blocked.
 To include every blocking event in the profile, pass rate = 1.
 To turn off profiling entirely, pass rate <= 0.

## Remote debugging

Assuming port forwarding is enabled on the local machine from port 17017 to the
debug server, a program profile can be obtained by running the following
command:

```
go tool pprof -seconds=5 http://localhost:17017/debug/pprof/profile
```

Please see [Profiling Go Programs](https://blog.golang.org/profiling-go-programs)
for further information on how to use the go pprof tool.

## License

debugserver is licensed under Apache 2.0.
