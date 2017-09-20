package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"runtime"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	gobuild "github.com/tonistiigi/llb-gobuild"
	"github.com/tonistiigi/llb-gobuild/loader"
)

type opt struct {
	target string
}

func main() {
	var o opt
	flag.StringVar(&o.target, "target", "/out/buildkit.llb.definition", "target file")
	flag.Parse()

	out := os.Stdout

	if o.target != "" {
		f, err := os.Create(o.target)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		out = f
	}

	var goOpt gobuild.BuildOptJSON
	if err := json.Unmarshal([]byte(os.Getenv("GOOPT")), &goOpt); err != nil {
		panic(err)
	}

	if goOpt.Pkg == "" {
		panic("no target pkg specified")
	}

	if goOpt.GOPATH == "" {
		goOpt.GOPATH = "/go"
	}

	if goOpt.GOOS == "" {
		goOpt.GOOS = runtime.GOOS
	}

	if goOpt.GOARCH == "" {
		goOpt.GOARCH = runtime.GOARCH
	}

	if err := generateLLB(goOpt, out); err != nil {
		panic(err)
	}
}

func generateLLB(opt gobuild.BuildOptJSON, out io.Writer) error {
	def, err := llb.ReadFrom(bytes.NewBuffer([]byte(opt.SourceDef)))
	if err != nil {
		return err
	}

	src := llb.NewState(&output{digest.Digest(opt.Source), opt.SourceIndex})

	l := loader.New(loader.GoOpt{
		Source:     src,
		MountPath:  opt.MountPath,
		CgoEnabled: opt.CgoEnabled,
		BuildTags:  opt.BuildTags,
		GOARCH:     opt.GOARCH,
		GOOS:       opt.GOOS,
		GOPATH:     opt.GOPATH,
	})

	st, err := l.BuildExe(opt.Pkg)
	if err != nil {
		return err
	}

	dt, err := st.Marshal()
	if err != nil {
		return err
	}

	def = append(def, dt...)

	// loadLLB(def)

	return llb.WriteTo(def, out)
}

type llbOp struct {
	Op     pb.Op
	Digest digest.Digest
}

func loadLLB(bs [][]byte) error {
	var ops []llbOp
	for _, dt := range bs {
		var op pb.Op
		if err := (&op).Unmarshal(dt); err != nil {
			return err
		}
		dgst := digest.FromBytes(dt)
		ops = append(ops, llbOp{Op: op, Digest: dgst})
	}
	enc := json.NewEncoder(os.Stdout)
	for _, op := range ops {
		if err := enc.Encode(op); err != nil {
			return err
		}
	}
	return nil
}

type output struct {
	dgst  digest.Digest
	index int
}

func (o *output) ToInput() (*pb.Input, error) {
	return &pb.Input{Digest: digest.Digest(o.dgst), Index: pb.OutputIndex(o.index)}, nil
}

func (o *output) Vertex() llb.Vertex {
	return &emptyVertex{}
}

type emptyVertex struct{}

func (v *emptyVertex) Validate() error {
	return nil
}
func (v *emptyVertex) Marshal() ([]byte, error) {
	return nil, nil
}
func (v *emptyVertex) Output() llb.Output {
	return nil
}
func (v *emptyVertex) Inputs() []llb.Output {
	return nil
}
