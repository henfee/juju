// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package featuretests

import (
	"github.com/juju/cmd/cmdtesting"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	"gopkg.in/juju/charm.v6"

	"github.com/juju/juju/cmd/juju/application"
	"github.com/juju/juju/cmd/juju/model"
	"github.com/juju/juju/constraints"
	"github.com/juju/juju/instance"
	jujutesting "github.com/juju/juju/juju/testing"
	"github.com/juju/juju/state"
)

// cmdJujuSuite tests the connectivity of juju commands.  These tests
// go from the command line, api client, api server, db. The db changes
// are then checked.  Only one test for each command is done here to
// check connectivity.  Exhaustive unit tests are at each layer.
type cmdJujuSuite struct {
	jujutesting.JujuConnSuite
}

func uint64p(val uint64) *uint64 {
	return &val
}

func (s *cmdJujuSuite) TestSetConstraints(c *gc.C) {
	_, err := cmdtesting.RunCommand(c, model.NewModelSetConstraintsCommand(), "mem=4G", "cpu-power=250")
	c.Assert(err, jc.ErrorIsNil)

	cons, err := s.State.ModelConstraints()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(cons, gc.DeepEquals, constraints.Value{
		CpuPower: uint64p(250),
		Mem:      uint64p(4096),
	})
}

func (s *cmdJujuSuite) TestGetConstraints(c *gc.C) {
	app := s.AddTestingApplication(c, "app", s.AddTestingCharm(c, "dummy"))
	err := app.SetConstraints(constraints.Value{CpuCores: uint64p(64)})
	c.Assert(err, jc.ErrorIsNil)

	context, err := cmdtesting.RunCommand(c, application.NewServiceGetConstraintsCommand(), "app")
	c.Assert(cmdtesting.Stdout(context), gc.Equals, "cores=64\n")
	c.Assert(cmdtesting.Stderr(context), gc.Equals, "")
}

func (s *cmdJujuSuite) TestServiceSet(c *gc.C) {
	ch := s.AddTestingCharm(c, "dummy")
	app := s.AddTestingApplication(c, "dummy-service", ch)

	_, err := cmdtesting.RunCommand(c, application.NewConfigCommand(), "dummy-service",
		"username=hello", "outlook=hello@world.tld")
	c.Assert(err, jc.ErrorIsNil)

	expect := charm.Settings{
		"username": "hello",
		"outlook":  "hello@world.tld",
	}

	settings, err := app.ConfigSettings()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(settings, gc.DeepEquals, expect)
}

func (s *cmdJujuSuite) TestServiceUnset(c *gc.C) {
	ch := s.AddTestingCharm(c, "dummy")
	app := s.AddTestingApplication(c, "dummy-service", ch)

	settings := charm.Settings{
		"username": "hello",
		"outlook":  "hello@world.tld",
	}

	err := app.UpdateConfigSettings(settings)
	c.Assert(err, jc.ErrorIsNil)

	_, err = cmdtesting.RunCommand(c, application.NewConfigCommand(), "dummy-service", "--reset", "username")
	c.Assert(err, jc.ErrorIsNil)

	expect := charm.Settings{
		"outlook": "hello@world.tld",
	}
	settings, err = app.ConfigSettings()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(settings, gc.DeepEquals, expect)
}

func (s *cmdJujuSuite) TestServiceGet(c *gc.C) {
	expected := `application: dummy-service
charm: dummy
settings:
  outlook:
    description: No default outlook.
    source: unset
    type: string
  skill-level:
    description: A number indicating skill.
    source: unset
    type: int
  title:
    default: My Title
    description: A descriptive title used for the application.
    source: default
    type: string
    value: My Title
  username:
    default: admin001
    description: The name of the initial account (given admin permissions).
    source: default
    type: string
    value: admin001
`
	ch := s.AddTestingCharm(c, "dummy")
	s.AddTestingApplication(c, "dummy-service", ch)

	context, err := cmdtesting.RunCommand(c, application.NewConfigCommand(), "dummy-service")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(cmdtesting.Stdout(context), jc.DeepEquals, expected)
}

func (s *cmdJujuSuite) TestServiceGetWeirdYAML(c *gc.C) {
	// This test has been confirmed to pass with the patch/goyaml-pr-241.diff
	// applied to the current gopkg.in/yaml.v2 revision, however since our standard
	// local test tooling doesn't apply patches, this test would fail without it.
	// When the goyaml has merged pr #241 and the dependencies updated, we can
	// remove the skip.
	c.Skip("Remove skip when goyaml has PR #241.")
	expected := `application: yaml-config
charm: yaml-config
settings:
  hexstring:
    default: true
    description: A hex string that should be a string, not a number.
    type: string
    value: "0xD06F00D"
  nonoctal:
    default: true
    description: Number that isn't valid octal, so should be a string.
    type: string
    value: 01182252
  numberstring:
    default: true
    description: A string that happens to contain a number.
    type: string
    value: "123456"
`
	ch := s.AddTestingCharm(c, "yaml-config")
	s.AddTestingApplication(c, "yaml-config", ch)

	context, err := cmdtesting.RunCommand(c, application.NewConfigCommand(), "yaml-config")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(cmdtesting.Stdout(context), jc.DeepEquals, expected)
}

func (s *cmdJujuSuite) TestServiceAddUnitExistingContainer(c *gc.C) {
	ch := s.AddTestingCharm(c, "dummy")
	app := s.AddTestingApplication(c, "some-application-name", ch)

	machine, err := s.State.AddMachine("quantal", state.JobHostUnits)
	c.Assert(err, jc.ErrorIsNil)
	template := state.MachineTemplate{
		Series: "quantal",
		Jobs:   []state.MachineJob{state.JobHostUnits},
	}
	container, err := s.State.AddMachineInsideMachine(template, machine.Id(), instance.LXD)
	c.Assert(err, jc.ErrorIsNil)

	_, err = cmdtesting.RunCommand(c, application.NewAddUnitCommand(), "some-application-name",
		"--to", container.Id())
	c.Assert(err, jc.ErrorIsNil)

	units, err := app.AllUnits()
	c.Assert(err, jc.ErrorIsNil)
	mid, err := units[0].AssignedMachineId()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(mid, gc.Equals, container.Id())
}
