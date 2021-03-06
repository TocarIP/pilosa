// Copyright 2017 Pilosa Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"bytes"
	"io/ioutil"
	"net"
	"strconv"
	"testing"

	"github.com/pilosa/pilosa/server"
	"github.com/pkg/errors"
)

func MustNewRunningServer(t *testing.T) *server.Command {
	s, err := newServer()
	if err != nil {
		t.Fatalf("getting new server: %v", err)
	}

	err = s.Run()
	if err != nil {
		t.Fatalf("running new pilosa server: %v", err)
	}
	return s
}

func newServer() (*server.Command, error) {
	s := server.NewCommand(&bytes.Buffer{}, ioutil.Discard, ioutil.Discard)

	port, err := findPort()
	if err != nil {
		return nil, errors.Wrap(err, "getting port")
	}
	s.Config.Bind = "localhost:" + strconv.Itoa(port)

	gport, err := findPort()
	if err != nil {
		return nil, errors.Wrap(err, "getting gossip port")
	}
	s.Config.GossipPort = strconv.Itoa(gport)

	s.Config.GossipSeed = "localhost:" + s.Config.GossipPort
	s.Config.Cluster.Type = "gossip"
	td, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "temp dir")
	}
	s.Config.DataDir = td
	return s, nil
}

func findPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", ":0")
	if err != nil {
		return 0, errors.Wrap(err, "resolving new port addr")
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, errors.Wrap(err, "listening to get new port")
	}
	port := l.Addr().(*net.TCPAddr).Port
	err = l.Close()
	if err != nil {
		return port, errors.Wrap(err, "closing listener")
	}
	return port, nil

}

func MustFindPort(t *testing.T) int {
	port, err := findPort()
	if err != nil {
		t.Fatalf("allocating new port: %v", err)
	}
	return port
}

type Cluster struct {
	Servers []*server.Command
}

func MustNewServerCluster(t *testing.T, size int) *Cluster {
	cluster, err := NewServerCluster(size)
	if err != nil {
		t.Fatalf("new cluster: %v", err)
	}
	return cluster
}

// **** below exists to have interface compatibility with cluster-resize,
// **** we can remove when cluster resize gets merged *****************//

type M struct {
	*server.Command
	Stdin  bytes.Buffer
	Stdout bytes.Buffer
	Stderr bytes.Buffer
}

func MustRunMainWithCluster(t *testing.T, size int) []*M {
	cluster := MustNewServerCluster(t, size)
	mains := make([]*M, 0)
	for _, s := range cluster.Servers {
		mains = append(mains, &M{Command: s})
	}
	return mains
}

// ***********************************************************************************//

func NewServerCluster(size int) (cluster *Cluster, err error) {
	cluster = &Cluster{
		Servers: make([]*server.Command, size),
	}
	hosts := make([]string, size)
	for i := 0; i < size; i++ {
		s, err := newServer()
		if err != nil {
			return nil, errors.Wrap(err, "new server")
		}
		cluster.Servers[i] = s
		hosts[i] = s.Config.Bind
		s.Config.GossipSeed = cluster.Servers[0].Config.GossipSeed

	}

	for _, s := range cluster.Servers {
		s.Config.Cluster.Hosts = hosts
	}
	for i, s := range cluster.Servers {
		err := s.Run()
		if err != nil {
			for j := 0; j <= i; j++ {
				cluster.Servers[j].Close()
			}
			return nil, errors.Wrapf(err, "starting server %d of %d. Config: %#v", i+1, size, s.Config)
		}
	}

	return cluster, nil
}
