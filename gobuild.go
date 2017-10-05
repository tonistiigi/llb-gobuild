package gobuild

import (
	"bytes"
	"encoding/json"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/llbbuild"
)

type Opt struct {
	DevMode bool
}

func New(opt *Opt) *GoBuilder {
	devMode := false
	if opt != nil && opt.DevMode {
		devMode = true
	}
	return &GoBuilder{DevMode: devMode}
}

type BuildOpt struct {
	Source     llb.State
	MountPath  string
	Pkg        string
	CgoEnabled bool
	BuildTags  []string
	GOARCH     string
	GOOS       string
}

type BuildOptJSON struct {
	Source      string
	SourceIndex int
	SourceDef   []byte
	MountPath   string
	Pkg         string
	CgoEnabled  bool
	BuildTags   []string
	GOARCH      string
	GOOS        string
	GOPATH      string
}

type GoBuilder struct {
	DevMode bool
}

func (gb *GoBuilder) BuildExe(opt BuildOpt) (*llb.State, error) {
	inp, err := opt.Source.Output().ToInput()
	if err != nil {
		return nil, err
	}

	def, err := opt.Source.Marshal()
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	if err := llb.WriteTo(def, buf); err != nil {
		return nil, err
	}

	dt, err := json.Marshal(BuildOptJSON{
		Source:      inp.Digest.String(),
		SourceIndex: int(inp.Index),
		SourceDef:   buf.Bytes(),
		MountPath:   opt.MountPath,
		Pkg:         opt.Pkg,
		CgoEnabled:  opt.CgoEnabled,
		BuildTags:   opt.BuildTags,
		GOARCH:      opt.GOARCH,
		GOOS:        opt.GOOS,
	})
	if err != nil {
		return nil, err
	}

	goBuild := llb.Image("docker.io/tonistiigi/llb-gobuild:latest@sha256:43da63157126f4156ddb8cef91c40b6a0de66d477fd3c413d28bc3c5141886bf")
	if gb.DevMode {
		goBuild = gobuildDev()
	}

	run := goBuild.Run(llb.Shlexf("gobuild %s", opt.Pkg), llb.AddEnv("GOOPT", string(dt)))
	run.AddMount(opt.MountPath, opt.Source, llb.Readonly)
	out := run.AddMount("/out", llb.Scratch()).With(llbbuild.Build())
	return &out, nil
}

func gobuildDev() llb.State {
	gobDev := llb.Local("gobuild-dev")
	build := goBuildBase().
		Run(llb.Shlex("apk add --no-cache git")).
		Dir("/go/src/github.com/tonistiigi/llb-gobuild").
		Run(llb.Shlex("sh -c \"go get -d github.com/moby/buildkit/client/llb && rm -rf /go/src/github.com/moby/buildkit/vendor/github.com/opencontainers/go-digest && go get -d github.com/opencontainers/go-digest\"")).
		Run(llb.Shlex("go build -o /out/gobuild github.com/tonistiigi/llb-gobuild/cmd/gobuild"))

	build.AddMount("/go/src/github.com/tonistiigi/llb-gobuild", gobDev, llb.Readonly)

	out := build.AddMount("/out", llb.Scratch())

	alpine := llb.Image("docker.io/library/alpine:latest")
	return copy(out, "/gobuild", alpine, "/bin")
}

func goBuildBase() llb.State {
	goAlpine := llb.Image("docker.io/library/golang:1.8-alpine@sha256:2287e0e274c1d2e9076c1f81d04f1a63c86b73c73603b09caada5da307a8f86d")
	return goAlpine.
		AddEnv("CGO_ENABLED", "0").
		AddEnv("GOPATH", "/go").
		AddEnv("PATH", "/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin") //.
	//AddEnv("GOPATH", "/go").Run(llb.Shlex("apk add --no-cache gcc libc-dev")).
	// Root()
}

// copy copies files between 2 states using cp until there is no copyOp
func copy(src llb.State, srcPath string, dest llb.State, destPath string) llb.State {
	cpImage := llb.Image("docker.io/library/alpine:latest")
	cp := cpImage.Run(llb.Shlexf("cp -a /src%s /dest%s", srcPath, destPath))
	cp.AddMount("/src", src, llb.Readonly)
	return cp.AddMount("/dest", dest)
}
