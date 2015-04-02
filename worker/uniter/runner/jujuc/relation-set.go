// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package jujuc

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/juju/cmd"
	"github.com/juju/errors"
	"github.com/juju/utils/keyvalues"
	"launchpad.net/gnuflag"
)

const relationSetDoc = `
"relation-set" writes the local unit's settings for some relation.
If no relation is specified then the current relation is used. The
setting values are not inspected and are stored as strings. Setting
an empty string causes the setting to be removed. Duplicate settings
are not allowed.

The --file option should be used when one or more key-value pairs are
too long to fit within the command length limit of the shell or
operating system. The file should contain key-value pairs in the same
format as on the commandline. They may also span multiple lines. Blank
lines and lines starting with # are ignored. Settings in the file will
be overridden by any duplicate key-value arguments.
`

// RelationSetCommand implements the relation-set command.
type RelationSetCommand struct {
	cmd.CommandBase
	ctx          Context
	RelationId   int
	Settings     map[string]string
	settingsFile string
	formatFlag   string // deprecated
}

func NewRelationSetCommand(ctx Context) cmd.Command {
	return &RelationSetCommand{ctx: ctx}
}

func (c *RelationSetCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "relation-set",
		Args:    "key=value [key=value ...]",
		Purpose: "set relation settings",
		Doc:     relationSetDoc,
	}
}

func (c *RelationSetCommand) SetFlags(f *gnuflag.FlagSet) {
	rV := newRelationIdValue(c.ctx, &c.RelationId)

	f.Var(rV, "r", "specify a relation by id")
	f.Var(rV, "relation", "")
	f.StringVar(&c.settingsFile, "file", "", "file containing key-value pairs")

	f.StringVar(&c.formatFlag, "format", "", "deprecated format flag")
}

func (c *RelationSetCommand) Init(args []string) error {
	if c.RelationId == -1 {
		return errors.Errorf("no relation id specified")
	}

	if err := c.handleSettings(args); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (c *RelationSetCommand) handleSettings(args []string) error {
	var settings map[string]string
	if c.settingsFile != "" {
		data, err := ioutil.ReadFile(c.settingsFile)
		if err != nil {
			return errors.Trace(err)
		}

		var kvs []string
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line[0] == '#' {
				continue
			}
			kvs = append(kvs, strings.Fields(line)...)
		}

		settings, err = keyvalues.Parse(kvs, true)
		if err != nil {
			return errors.Trace(err)
		}
	} else {
		settings = make(map[string]string)
	}

	overrides, err := keyvalues.Parse(args, true)
	if err != nil {
		return errors.Trace(err)
	}
	for k, v := range overrides {
		settings[k] = v
	}

	c.Settings = settings
	return nil
}

func (c *RelationSetCommand) Run(ctx *cmd.Context) (err error) {
	if c.formatFlag != "" {
		fmt.Fprintf(ctx.Stderr, "--format flag deprecated for command %q", c.Info().Name)
	}
	r, found := c.ctx.Relation(c.RelationId)
	if !found {
		return fmt.Errorf("unknown relation id")
	}
	settings, err := r.Settings()
	if err != nil {
		return errors.Annotate(err, "cannot read relation settings")
	}
	for k, v := range c.Settings {
		if v != "" {
			settings.Set(k, v)
		} else {
			settings.Delete(k)
		}
	}
	return nil
}
