// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package buildcfg

import (
	"fmt"
	"internal/kibou_expt"
	"maps"
	"reflect"
	"slices"
	"strings"
)

func parseKibouExperiments(flags *ExperimentFlags, kibouExpts string) error {
	names := make(map[string]func(bool))
	rv := reflect.ValueOf(&flags.Kibou).Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rv.Field(i)
		names[rt.Field(i).Name] = field.SetBool
	}
	unsupported := map[string]struct{}{}
	for f := range strings.SplitSeq(kibouExpts, ",") {
		if f == "" {
			continue
		}
		if f == "none" {
			// KIBOU_EXPERIMENTS=none disables all experiment flags.
			// This is used by cmd/dist, which doesn't know how
			// to build with any experiment flags.
			flags.Kibou = kibou_expt.Flags{}
			continue
		}
		val := true
		if strings.HasPrefix(f, "No") {
			f, val = f[2:], false
		}
		set, ok := names[f]
		if !ok {
			unsupported[f] = struct{}{}
			continue
		}
		set(val)
	}
	if len(unsupported) == 0 {
		return nil
	}
	unknownExpts := slices.Sorted(maps.Keys(unsupported))
	val := "values"
	if len(unknownExpts) == 1 {
		val = "value"
	}
	return fmt.Errorf("unsupported %s for KIBOU_EXPERIMENTS %s\nSupported value(s): %s",
		val, strings.Join(unknownExpts, ", "),
		strings.Join(slices.Sorted(maps.Keys(names)), ", "))
}

// kibouExpList returns the list of PascalCase experiment names for
// experiments that differ from base. base may be nil to indicate no
// experiments. If all is true, then include all experiment flags,
// regardless of base.
func kibouExpList(exp, base *kibou_expt.Flags, all bool, out *[]string) {
	rv := reflect.ValueOf(exp).Elem()
	var rBase reflect.Value
	if base != nil {
		rBase = reflect.ValueOf(base).Elem()
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		name := rt.Field(i).Name // don't lowercase here, unlike expList
		val := rv.Field(i).Bool()
		baseVal := false
		if base != nil {
			baseVal = rBase.Field(i).Bool()
		}
		if all || val != baseVal {
			if val {
				*out = append(*out, name)
			} else {
				*out = append(*out, "No"+name)
			}
		}
	}
}
