package main

import (
	"os/exec"
	"reflect"
	"testing"
)


func TestExpand(t *testing.T) {
	cmds := cmdMap{
		"s3": func(flag string, arg string) (string, error) {
			return "s3(" + arg + ")", nil
		},
	}
	
	tests := []struct{
		cmdLine []string
		cmd *exec.Cmd
	} {
		{
			[]string{"--", "tree"},
			exec.Command("tree"),
		},
		{
			[]string{"--", "tree", "--a", "--b=c"},
			exec.Command("tree", "--a", "--b=c"),
		},
		{
			[]string{"--", "tree", "--a", "--b", "c"},
			exec.Command("tree", "--a", "--b", "c"),
		},
		{
			[]string{"a=s3:A-BUCKET-ID", "--", "tree", "--a=%a", "--b", "c"},
			exec.Command("tree", "--a=s3(A-BUCKET-ID)", "--b", "c"),
		},
		{
			[]string{"a=s3:A-BUCKET-ID", "--", "tree", "--%a", "--b", "c"},
			exec.Command("tree", "--a=s3(A-BUCKET-ID)", "--b", "c"),
		},
		{
			[]string{"e=s3:A", "e=s3:B", "--", "tree", "--e=%e", "--b", "c", "--e", "%e", "-x", "--e=%e"},
			exec.Command("tree", "--e=s3(A)", "--b", "c", "--e", "s3(B)", "-x", "--e=s3(A)"),
		},
		{
			[]string{"a=s3:A", "name=s3:hello", "--", "tree", "%name", "--a=%a"},
			exec.Command("tree", "s3(hello)", "--a=s3(A)"),
		},
		{
			[]string{"a=s3:A", "name=s3:hello", "--", "tree", "--x", "%name"},
			exec.Command("tree", "--x", "s3(hello)"),
		},
	}

	for _, tc := range tests {
		t.Logf("%+v", tc)
		cmd, err := expand(tc.cmdLine, cmds)
		if err != nil {
			t.Fatal(err)
		}
		t.Log("g", cmd.Path)
		t.Log("g", cmd.Args)

		if cmd.Path != tc.cmd.Path {
			t.Log("w", tc.cmd.Path)
			t.Fatal("bad path")
		}

		if !reflect.DeepEqual(cmd.Args, tc.cmd.Args) {
			t.Log("w", tc.cmd.Args)
			t.Fatal("bad args")
		}
		t.Log("OK")
	}
}
