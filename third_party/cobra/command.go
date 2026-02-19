package cobra

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

type Command struct {
	Use               string
	Short             string
	Run               func(cmd *Command, args []string)
	RunE              func(cmd *Command, args []string) error
	PersistentPreRunE func(cmd *Command, args []string) error
	SilenceUsage      bool

	out io.Writer
	err io.Writer

	parent   *Command
	children []*Command

	localFlags      *FlagSet
	persistentFlags *FlagSet
}

func (c *Command) AddCommand(commands ...*Command) {
	for _, child := range commands {
		child.parent = c
		c.children = append(c.children, child)
	}
}

func (c *Command) SetOut(w io.Writer) {
	c.out = w
}

func (c *Command) SetErr(w io.Writer) {
	c.err = w
}

func (c *Command) OutOrStdout() io.Writer {
	if c.out != nil {
		return c.out
	}
	if c.parent != nil {
		return c.parent.OutOrStdout()
	}
	return os.Stdout
}

func (c *Command) ErrOrStderr() io.Writer {
	if c.err != nil {
		return c.err
	}
	if c.parent != nil {
		return c.parent.ErrOrStderr()
	}
	return os.Stderr
}

func (c *Command) Flags() *FlagSet {
	if c.localFlags == nil {
		c.localFlags = newFlagSet(c.Use)
	}
	return c.localFlags
}

func (c *Command) PersistentFlags() *FlagSet {
	if c.persistentFlags == nil {
		c.persistentFlags = newFlagSet(c.Use)
	}
	return c.persistentFlags
}

func (c *Command) MarkFlagRequired(name string) error {
	c.Flags().markRequired(name)
	return nil
}

func (c *Command) Execute() error {
	return c.execute(os.Args[1:])
}

func (c *Command) execute(args []string) error {
	if helpRequested(args) {
		c.printHelp()
		return nil
	}

	if child, rest := c.findChild(args); child != nil {
		remaining, err := c.PersistentFlags().parseKnown(rest)
		if err != nil {
			return err
		}
		if c.PersistentPreRunE != nil {
			if err := c.PersistentPreRunE(c, nil); err != nil {
				return err
			}
		}
		return child.execute(remaining)
	}

	remaining, err := c.PersistentFlags().parseKnown(args)
	if err != nil {
		return err
	}
	if c.PersistentPreRunE != nil {
		if err := c.PersistentPreRunE(c, nil); err != nil {
			return err
		}
	}

	remaining, err = c.Flags().parseStrict(remaining)
	if err != nil {
		return err
	}
	if err := c.Flags().validateRequired(); err != nil {
		return err
	}

	switch {
	case c.RunE != nil:
		return c.RunE(c, remaining)
	case c.Run != nil:
		c.Run(c, remaining)
		return nil
	case len(c.children) > 0:
		c.printHelp()
		return nil
	default:
		return errors.New("comando no ejecutable")
	}
}

func (c *Command) findChild(args []string) (*Command, []string) {
	for i, arg := range args {
		if len(arg) > 0 && arg[0] == '-' {
			continue
		}
		for _, child := range c.children {
			if arg == child.Use {
				rest := append([]string{}, args[:i]...)
				rest = append(rest, args[i+1:]...)
				return child, rest
			}
		}
	}
	return nil, args
}

func (c *Command) printHelp() {
	fmt.Fprintf(c.OutOrStdout(), "%s\n", c.Short)
	fmt.Fprintf(c.OutOrStdout(), "\nUso:\n  %s\n", c.Use)
	if len(c.children) > 0 {
		fmt.Fprintln(c.OutOrStdout(), "\nComandos:")
		for _, child := range c.children {
			fmt.Fprintf(c.OutOrStdout(), "  %-15s %s\n", child.Use, child.Short)
		}
	}
}

func helpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

type FlagSet struct {
	set      *flag.FlagSet
	required map[string]bool
}

func newFlagSet(name string) *FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	return &FlagSet{
		set:      f,
		required: map[string]bool{},
	}
}

func (f *FlagSet) StringVar(target *string, name string, value string, usage string) {
	f.set.StringVar(target, name, value, usage)
}

func (f *FlagSet) BoolVar(target *bool, name string, value bool, usage string) {
	f.set.BoolVar(target, name, value, usage)
}

func (f *FlagSet) parseStrict(args []string) ([]string, error) {
	if f == nil {
		return nil, nil
	}
	if err := f.set.Parse(args); err != nil {
		return nil, err
	}
	return f.set.Args(), nil
}

func (f *FlagSet) parseKnown(args []string) ([]string, error) {
	if f == nil {
		return nil, nil
	}

	selected := make([]string, 0, len(args))
	remaining := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "--" {
			remaining = append(remaining, args[i:]...)
			break
		}
		if !strings.HasPrefix(token, "-") {
			remaining = append(remaining, token)
			continue
		}

		name, hasValueInline := parseFlagName(token)
		flg := f.set.Lookup(name)
		if flg == nil {
			remaining = append(remaining, token)
			if !hasValueInline && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				remaining = append(remaining, args[i+1])
				i++
			}
			continue
		}

		selected = append(selected, token)
		if hasValueInline || isBoolFlag(flg) {
			continue
		}
		if i+1 < len(args) {
			selected = append(selected, args[i+1])
			i++
		}
	}

	if err := f.set.Parse(selected); err != nil {
		return nil, err
	}
	remaining = append(remaining, f.set.Args()...)
	return remaining, nil
}

func parseFlagName(token string) (name string, hasValueInline bool) {
	trimmed := strings.TrimLeft(token, "-")
	parts := strings.SplitN(trimmed, "=", 2)
	if len(parts) == 2 {
		return parts[0], true
	}
	return parts[0], false
}

func isBoolFlag(f *flag.Flag) bool {
	type boolFlag interface {
		IsBoolFlag() bool
	}
	bf, ok := f.Value.(boolFlag)
	return ok && bf.IsBoolFlag()
}

func (f *FlagSet) markRequired(name string) {
	f.required[name] = true
}

func (f *FlagSet) validateRequired() error {
	if f == nil {
		return nil
	}
	for name := range f.required {
		flg := f.set.Lookup(name)
		if flg == nil {
			return fmt.Errorf("flag requerida no encontrada: --%s", name)
		}
		if flg.Value.String() == flg.DefValue {
			return fmt.Errorf("falta flag requerida: --%s", name)
		}
	}
	return nil
}
