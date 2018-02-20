package main

import (
	"os/exec"
	"reflect"
	"testing"
)

func TestExpand(t *testing.T) {
	cmds := cmdMap{
		"s3": func(arg string) (string, error) {
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
			[]string{"--a=s3:A-BUCKET-ID", "--", "tree", "--a", "--b", "c"},
			exec.Command("tree", "--a=s3(A-BUCKET-ID)", "--b", "c"),
		},
		{
			[]string{"--a", "s3:A-BUCKET-ID", "--", "tree", "--a", "--b", "c"},
			exec.Command("tree", "--a", "s3(A-BUCKET-ID)", "--b", "c"),
		},
		{
			[]string{"--d", "s3:A-BUCKET-ID", "--", "tree", "--a", "--b", "c", "--d"},
			exec.Command("tree", "--a", "--b", "c", "--d", "s3(A-BUCKET-ID)"),
		},
		{
			[]string{"--d=s3:A-BUCKET-ID", "--", "tree", "--a", "--b", "c", "--d"},
			exec.Command("tree", "--a", "--b", "c", "--d=s3(A-BUCKET-ID)"),
		},
	}

	for _, tc := range tests {
		t.Log(tc.cmdLine)
		cmd, err := expand(tc.cmdLine, cmds)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(cmd.Path)
		t.Log(cmd.Args)

		if cmd.Path != tc.cmd.Path {
			t.Log("want", tc.cmd.Path)
			t.Fatal()
		}

		if !reflect.DeepEqual(cmd.Args, tc.cmd.Args) {
			t.Log("want", tc.cmd.Args)
			t.Fatal()
		}
	}
}
