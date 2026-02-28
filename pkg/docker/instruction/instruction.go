// Package instruction describes the Docker instruction data model.
package instruction

import (
	df "github.com/mintoolkit/mint/pkg/docker/dockerfile"
	"strings"
)

type Field struct {
	GlobalIndex int      `json:"start_index"`
	StageIndex  int      `json:"stage_index"`
	StageID     int      `json:"stage_id"`
	RawData     string   `json:"-"`
	RawLines    []string `json:"raw_lines"`
	StartLine   int      `json:"start_line"`
	EndLine     int      `json:"end_line"`
	Name        string   `json:"name"`
	Flags       []string `json:"flags,omitempty"`
	Args        []string `json:"args,omitempty"`
	ArgsRaw     string   `json:"args_raw,omitempty"`
	IsJSONForm  bool     `json:"is_json"`
	IsOnBuild   bool     `json:"is_onbuild,omitempty"`
	IsValid     bool     `json:"is_valid"`
	Errors      []string `json:"errors,omitempty"`
}

type Format struct {
	Name               string
	SupportsFlags      bool //todo: add a list allowed flags
	SupportsJSONForm   bool
	SupportsNameValues bool
	RequiresNameValues bool
	SupportsSubInst    bool
	IsDeprecated       bool
}

// Specs is a map of all available instructions and their format info (by name)
var Specs = map[string]Format{
	df.InstTypeAdd: {
		Name:             df.InstTypeAdd,
		SupportsFlags:    true,
		SupportsJSONForm: true,
	},
	df.InstTypeArg: {
		Name:               df.InstTypeArg,
		SupportsNameValues: true,
	},
	df.InstTypeCmd: {
		Name:             df.InstTypeCmd,
		SupportsJSONForm: true,
	},
	df.InstTypeCopy: {
		Name:             df.InstTypeCopy,
		SupportsFlags:    true,
		SupportsJSONForm: true,
	},
	df.InstTypeEntrypoint: {
		Name:             df.InstTypeEntrypoint,
		SupportsJSONForm: true,
	},
	df.InstTypeEnv: {
		Name:               df.InstTypeEnv,
		RequiresNameValues: true,
	},
	df.InstTypeExpose: {
		Name: df.InstTypeExpose,
	},
	df.InstTypeFrom: {
		Name:          df.InstTypeFrom,
		SupportsFlags: true,
	},
	df.InstTypeHealthcheck: {
		Name:             df.InstTypeHealthcheck,
		SupportsJSONForm: true,
	},
	df.InstTypeLabel: {
		Name:               df.InstTypeLabel,
		RequiresNameValues: true,
	},
	df.InstTypeMaintainer: {
		Name:         df.InstTypeMaintainer,
		IsDeprecated: true,
	},
	df.InstTypeOnbuild: {
		Name:            df.InstTypeLabel,
		SupportsSubInst: true,
	},
	df.InstTypeRun: {
		Name:             df.InstTypeRun,
		SupportsJSONForm: true,
	},
	df.InstTypeShell: {
		Name:             df.InstTypeShell,
		SupportsJSONForm: true,
	},
	df.InstTypeStopSignal: {
		Name: df.InstTypeStopSignal,
	},
	df.InstTypeUser: {
		Name: df.InstTypeUser,
	},
	df.InstTypeVolume: {
		Name:             df.InstTypeVolume,
		SupportsJSONForm: true,
	},
	df.InstTypeWorkdir: {
		Name: df.InstTypeWorkdir,
	},
}

func IsKnown(name string) bool {
	name = strings.ToUpper(name)
	_, ok := Specs[name]
	return ok
}

func SupportsJSONForm() []string {
	var names []string
	for _, spec := range Specs {
		names = append(names, spec.Name)
	}

	return names
}
