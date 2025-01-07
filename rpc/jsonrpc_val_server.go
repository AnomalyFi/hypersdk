package rpc

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
)

type JSONRPCValServerConfig struct {
	// when DerivePort is true, derive the port from nodeID, otherwise use specified
	DerivePort bool `json:"derivePort"`
	Port       int  `json:"port"`
}

var valSerName = "val"

type JSONRPCValServer struct {
	vm VM
}

func NewJSONRPCValServer(vm VM) *JSONRPCValServer {
	return &JSONRPCValServer{vm}
}

func (j *JSONRPCValServer) Serve(port string) {
	s := rpc.NewServer()
	codec := json.NewCodec()
	s.RegisterCodec(json.NewCodec(), "application/json")
	s.RegisterCodec(codec, "application/json;charset=UTF-8")

	s.RegisterService(j, valSerName) //nolint:errcheck
	r := mux.NewRouter()
	r.Handle("/", s)

	server := &http.Server{
		Addr:         port,
		Handler:      r,
		ReadTimeout:  5 * time.Second,  // Time limit for reading request
		WriteTimeout: 5 * time.Second,  // Time limit for writing response
		IdleTimeout:  10 * time.Second, // Time limit for idle connections
	}

	_ = server.ListenAndServe()
}

type UpdateArcadiaURLArgs struct {
	URL string `json:"url"`
}

func (*JSONRPCValServer) Ping(_ *http.Request, _ *struct{}, reply *PingReply) error {
	reply.Success = true
	return nil
}

func (j *JSONRPCValServer) UpdateArcadiaURL(_ *http.Request, args *UpdateArcadiaURLArgs, _ *struct{}) error {
	return j.vm.ReplaceArcadia(args.URL)
}

// SetMempoolExemptSponsors is mocked for now. Revist after hypersdk upgrade.
func (*JSONRPCValServer) SetMempoolExemptSponsors(_ *http.Request, _ *struct{}, _ *struct{}) error {
	return nil
}
