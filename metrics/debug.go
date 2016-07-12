package metrics

import (
	"expvar"
	"net/http"
	"strings"

	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/http_server"
)

func StartDebugServer(address string, sink *lager.ReconfigurableSink, metrics Metrics) (ifrit.Process, error) {
	expvar.Publish("numCPUS", expvar.Func(func() interface{} {
		return metrics.NumCPU()
	}))

	expvar.Publish("numGoRoutines", expvar.Func(func() interface{} {
		return metrics.NumGoroutine()
	}))

	expvar.Publish("loopDevices", expvar.Func(func() interface{} {
		return metrics.LoopDevices()
	}))

	expvar.Publish("backingStores", expvar.Func(func() interface{} {
		return metrics.BackingStores()
	}))

	expvar.Publish("depotDirs", expvar.Func(func() interface{} {
		return metrics.DepotDirs()
	}))

	server := http_server.New(address, handler(sink))
	p := ifrit.Invoke(server)
	select {
	case <-p.Ready():
	case err := <-p.Wait():
		return nil, err
	}
	return p, nil
}

func handler(sink *lager.ReconfigurableSink) http.Handler {
	pprofHandler := debugserver.Handler(sink)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/debug/vars") {
			http.DefaultServeMux.ServeHTTP(w, r)
			return
		}
		pprofHandler.ServeHTTP(w, r)
	})
}
