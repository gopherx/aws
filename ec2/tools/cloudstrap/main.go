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

type cmdFunc func(flag string, arg string) (string, error)

type cmdMap map[string]cmdFunc

var (
	cmds = map[string]cmdFunc{
		"s3":            s3download,
		"http_get":      bytes2string(httpGet),
		"file_http_get": bytes2file(httpGet),
	}
)

func s3download(flag string, id string) (string, error) {
	return "", nil
}

func bytes2string(fn func(flag string, url string) ([]byte, error)) func(flag string, url string) (string, error) {
	return func(flag string, url string) (string, error) {
		b, err := fn(flag, url)
		return string(b), err
	}
}

func httpGet(flag string, url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.InvalidArgument(nil, "GET failed", url, resp.StatusCode, resp)
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func bytes2file(fn func(flag string, url string) ([]byte, error)) func(flag string, url string) (string, error) {
	return func(flag string, url string) (string, error) {
		b, err := fn(flag, url)
		if err != nil {
			return "", err
		}

		f, err := ioutil.TempFile(".", flag)
		if err != nil {
			return "", err
		}

		return f.Name(), ioutil.WriteFile(f.Name(), b, 0644)
	}
}

type expando struct {
	//flag flag.Spec
	name string
	cmd  cmdFunc
	arg  string
}

func (e *expando) expand() (string, error) {
	result, err := e.cmd(e.name, e.arg)
	if err != nil {
		return "", err
	}
	return result, nil
}

func scan(args []string, cmds cmdMap) ([]string, []*expando, error) {
	exps := []*expando{}

	cmd, err := flag.Scan(args, func(f flag.Spec) error {
		if len(f.Header) > 0 || f.Separator != "=" {
			return errors.InvalidArgument(nil, "only key=cmd:arg arguments are supported", f)
		}

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
		exps = append(exps, &expando{f.Name, cmd, arg})
		return nil
	})

	return cmd, exps, err
}

func appendFlag(args []string, f flag.Spec) []string {
	if len(f.Header) == 0 {
		//...a flag without prefix; only append the value
		return append(args, f.Value)
	}

	//...a normal '-' prefixed flag.
	tmp := fmt.Sprint(f.Header, f.Name)
	if f.Separator == "=" {
		tmp = fmt.Sprint(tmp, "=", f.Value)
	}

	args = append(args, tmp)

	if f.Separator != "=" && len(f.Value) > 0 {
		args = append(args, f.Value)
	}
	return args
}

// buildCmd generates the command from the flags found in rem and the provided expandos.
// The rem varable may contain the same flag multiple times; the assigned value will be
// the matching expando until all expandos are used when this happens the values from the
// expandos will be used in a round robin fashion.
func buildCmd(rem []string, exps expandoMap) (*exec.Cmd, error) {
	args := []string{}
	_, err := flag.Scan(rem[1:], func(f flag.Spec) error {
		if len(f.Header) > 0 && !strings.HasPrefix(f.Name, "%") && strings.HasPrefix(f.Value, "%") {
			// '--x=%name' type expansion
			eb, ok := exps[strings.TrimPrefix(f.Value, "%")]
			if !ok {
				return errors.InvalidArgument(nil, "unknown key", f)
			}

			ef := f
			var err error
			ef.Value, err = eb.next().expand()
			if err != nil {
				return err
			}
			args = appendFlag(args, ef)
			return nil
		}

		if len(f.Header) > 0 && !strings.HasPrefix(f.Name, "%") && !strings.HasPrefix(f.Value, "%") {
			// '--x or --x foo or --x=foo' type flag
			args = appendFlag(args, f)
			return nil
		}

		if len(f.Header) == 0 && strings.HasPrefix(f.Name, "%") && len(f.Value) == 0 {
			// '%x' type expansion
			eb, ok := exps[strings.TrimPrefix(f.Name, "%")]
			if !ok {
				return errors.InvalidArgument(nil, "unknown key", f)
			}

			v, err := eb.next().expand()
			if err != nil {
				return err
			}

			args = append(args, v)
			return nil
		}

		if len(f.Header) > 0 && strings.HasPrefix(f.Name, "%") && len(f.Value) == 0 {
			// '--%x' type shortcut expansion
			name := strings.TrimPrefix(f.Name, "%")
			eb, ok := exps[name]
			if !ok {
				return errors.InvalidArgument(nil, "unknown key", f)
			}

			ef := f
			ef.Name = name
			ef.Separator = "="
			var err error
			ef.Value, err = eb.next().expand()
			if err != nil {
				return err
			}
			args = appendFlag(args, ef)
			return nil
		}

		return errors.InvalidArgument(nil, "unsupported format", f, len(f.Value))
	})

	cmd := exec.Command(rem[0], args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, err
}

type expandoBag struct {
	expandos []*expando
	n        int
}

func (e *expandoBag) next() *expando {
	cur := e.expandos[e.n%len(e.expandos)]
	e.n++
	return cur
}

type expandoMap map[string]*expandoBag

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
		tmp, ok := m[exp.name]
		if !ok {
			tmp = &expandoBag{}
			m[exp.name] = tmp
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
