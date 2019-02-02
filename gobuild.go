package gobuild

import (
	"bytes"
	"encoding/json"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/llbbuild"
	"github.com/moby/buildkit/solver/pb"
	"github.com/pkg/errors"
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
	def, err := opt.Source.Marshal()
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	if err := llb.WriteTo(def, buf); err != nil {
		return nil, err
	}
	var op pb.Op
	if err := (&op).Unmarshal(def.Def[len(def.Def)-1]); err != nil {
		return nil, errors.Wrap(err, "failed to parse llb proto op")
	}
	if len(op.Inputs) == 0 {
		return nil, errors.Errorf("invalid source state")
	}

	dt, err := json.Marshal(BuildOptJSON{
		Source:      op.Inputs[0].Digest.String(),
		SourceIndex: int(op.Inputs[0].Index),
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

	goBuild := llb.Image("docker.io/tonistiigi/llb-gobuild@sha256:511744f1570cfc9c88e67fb88181986f19841c6880aafa6fe8d5a3f6e7144f61")
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
		Dir("/src").
		Run(llb.Shlex("go build -o /out/gobuild ./cmd/gobuild"))

	build.AddMount("/src", gobDev, llb.Readonly)
	build.AddMount("/go/pkg/mod", llb.Scratch(), llb.AsPersistentCacheDir("gocache", llb.CacheMountShared))

	out := build.AddMount("/out", llb.Scratch())

	alpine := llb.Image("docker.io/library/alpine:latest")
	return copy(out, "/gobuild", alpine, "/bin")
}

func goBuildBase() llb.State {
	goAlpine := llb.Image("docker.io/library/golang:1.11-alpine@sha256:31389db6001c5222bef9817a04ae8c8401ae8bed6fb965aac17b0e742f4c3e5e")
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
