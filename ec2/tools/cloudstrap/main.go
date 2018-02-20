package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/gopherx/base/errors"
	"github.com/gopherx/base/flag"
)

type cmdFunc func(arg string) (string, error)

type cmdMap map[string]cmdFunc

var (
	cmds = map[string]cmdFunc{
		"s3":  s3download,
		"GET": httpGet,
	}
)

func s3download(id string) (string, error) {
	return "", nil
}

func httpGet(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", errors.InvalidArgument(nil, "GET failed", url, resp.StatusCode, resp)
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

type expando struct {
	flag  flag.Spec
	cmd   cmdFunc
	arg   string
}

func (e *expando) expand() (string, error) {
	result, err := e.cmd(e.arg)
	if err != nil {
		return "", err
	}
	return result, nil
}

func scan(args []string, cmds cmdMap) ([]string, []*expando, error) {
	exps := []*expando{}

	cmd, err := flag.Scan(args, func(f flag.Spec) error {
		sepAt := strings.IndexByte(f.Value, ':')
		if sepAt < 0 {
			return errors.InvalidArgument(nil, "malformed flag; no command specified", f)
		}

		cmdID := f.Value[:sepAt]
		cmd, ok := cmds[cmdID]
		if !ok {
			return errors.InvalidArgument(nil, "unknown command", cmdID)
		}

		arg := f.Value[sepAt+1:]
		exps = append(exps, &expando{f, cmd, arg})
		return nil
	})

	return cmd, exps, err
}

func appendFlag(args []string, f flag.Spec) []string {
	tmp := fmt.Sprint(f.Header, f.Name)
	if f.Separator == "=" {
		tmp = fmt.Sprint(tmp, "=", f.Value)
	}

	args = append(args, tmp)

	if f.Separator == " " {
		args = append(args, f.Value)
	}
	return args
}

func appendExpando(args []string, e *expando) ([]string, error) {
	res, err := e.expand()
	if err != nil {
		return nil, err
	}

	f := e.flag
	f.Value = res
	return appendFlag(args, f), nil
}

// buildCmd generates the command from the flags found in rem and the provided expandos.
// The rem varable may contain the same flag multiple times; the assigned value will be
// the matching expando until all expandos are used when this happens the values from the
// expandos will be used in a round robin fashion.
func buildCmd(rem []string, exps expandoMap) (*exec.Cmd, error) {	
	args := []string{}
	_, err := flag.Scan(rem[1:], func(f flag.Spec) error {
		var err error
		b, ok := exps[f.Name]
		if !ok {
			args = appendFlag(args, f)
			return nil
		}

		args, err = appendExpando(args, b.next())
		return err
	})

	cmd := exec.Command(rem[0], args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr	
	return cmd, err
}

type expandoBag struct {
	expandos []*expando
	n int
}

type expandoMap map[string]*expandoBag

func (e *expandoBag) next() *expando {
	cur := e.expandos[e.n % len(e.expandos)]
	e.n++
	return cur
}

func expand(args []string, cmds cmdMap) (*exec.Cmd, error) {
	rem, exps, err := scan(args, cmds)
	if err != nil {
		return nil, err
	}

	if len(rem) == 0 {
		return nil, errors.InvalidArgument(nil, "no program to execute", args)
	}

	m := expandoMap{}
	for _, exp := range exps {
		tmp, ok := m[exp.flag.Name]
		if !ok {
			tmp = &expandoBag{}
			m[exp.flag.Name] = tmp
		}

		tmp.expandos = append(tmp.expandos, exp)
	}

	cmd, err := buildCmd(rem, m)
	return cmd, err
}

func main() {
	fmt.Println(os.Args)
	cmd, err := expand(os.Args[1:], cmds)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(cmd.Args)
	cmd.Run()
}
