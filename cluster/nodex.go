// Copyright (c) nano Authors. All Rights Reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cluster

import (
	"expvar"
	"html/template"
	"net"
	"net/http"
	"net/http/pprof"
	"sort"
	"strconv"

	"github.com/aclisp/go-nano/internal/log"
	"github.com/aclisp/go-nano/session"
)

func (n *Node) startMonitor() {
	if n.MonitorAddr == "" {
		n.MonitorAddr = determineMonitorAddr(n.ServiceAddr)
	}
	if n.MonitorAddr == "" {
		log.Print("Can not start node monitor")
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/vars", expvar.Handler())
	mux.HandleFunc("/debug/nano/node", n.nodeInfo)

	go func() {
		if len(n.TSLCertificate) != 0 {
			log.Fatal(http.ListenAndServeTLS(n.MonitorAddr, n.TSLCertificate, n.TSLKey, mux))
		} else {
			log.Fatal(http.ListenAndServe(n.MonitorAddr, mux))
		}
	}()

	monitorURL := "http://" + n.MonitorAddr
	if len(n.TSLCertificate) != 0 {
		monitorURL = "https://" + n.MonitorAddr
	}
	log.Print("Node monitor running at", monitorURL)
}

func (n *Node) Members() []*Member {
	n.cluster.mu.RLock()
	defer n.cluster.mu.RUnlock()
	return n.cluster.members
}

func (n *Node) Sessions() []*session.Session {
	n.mu.RLock()
	var result = make([]*session.Session, 0, len(n.sessions))
	for _, s := range n.sessions {
		result = append(result, s)
	}
	n.mu.RUnlock()

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID() < result[j].ID()
	})
	return result
}

func determineMonitorAddr(serviceAddr string) (monitorAddr string) {
	// ignore err here because serviceAddr should be validated
	host, port, _ := net.SplitHostPort(serviceAddr)
	portnum, _ := strconv.Atoi(port)
	const numPortScan = 10
	for offset := 1; offset <= numPortScan; offset++ {
		monitorAddr = net.JoinHostPort(host, strconv.Itoa(portnum+offset))
		if listener, err := net.Listen("tcp", monitorAddr); err == nil {
			listener.Close()
			return monitorAddr
		}
	}
	return ""
}

func (n *Node) shrinkRPCClient() {
	n.rpcClient.shrinkTo(n.cluster.remoteAddrs())
}

func (n *Node) nodeInfo(w http.ResponseWriter, r *http.Request) {
	const tmplPath = "./tmpl/"
	nodeTmpl, err := template.ParseFiles(
		tmplPath+"node.html",
		tmplPath+"components.html",
		tmplPath+"remotes.html",
		tmplPath+"members.html",
		tmplPath+"sessions.html",
		tmplPath+"mesh.html",
	)
	if err != nil {
		log.Print(err)
		return
	}
	if err := nodeTmpl.Execute(w, n); err != nil {
		log.Print(err)
		return
	}
}
