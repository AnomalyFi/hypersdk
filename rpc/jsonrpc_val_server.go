package rpc

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
)

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

	s.RegisterService(new(JSONRPCValServer), valSerName)
	r := mux.NewRouter()
	r.Handle("/", s)
	http.ListenAndServe(port, r)
}

type UpdateArcadiaURLArgs struct {
	Url string `json:"url"`
}

func (j *JSONRPCValServer) Ping(_ *http.Request, _ *struct{}, reply *PingReply) (err error) {
	reply.Success = true
	return nil
}

func (j *JSONRPCValServer) UpdateArcadiaURL(_ *http.Request, args *UpdateArcadiaURLArgs, _ *struct{}) error {
	j.vm.ReplaceArcadia(args.Url)
	return nil
}

func (j *JSONRPCValServer) SetMempoolExemptSponsors(_ *http.Request, _ *struct{}, _ *struct{}) error {
	return nil
}
