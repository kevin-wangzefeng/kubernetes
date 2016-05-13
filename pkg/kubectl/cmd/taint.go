/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	utilerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/validation"
	"encoding/json"
)

const (
	taint_long = `Update the taints on a resource.

A taint must begin with a letter or number, and may contain letters, numbers, hyphens, dots, and underscores, up to %[1]d characters.`
	taint_example = `# Update node 'foo' with a taint with key 'dedicated' and value 'special-user' and effect 'NoScheduleNoAdmitNoExecute'.
# If a taint with that key already exists, its value and effect are replaced as specified.
kubectl taint nodes foo dedicated=special-user:NoScheduleNoAdmitNoExecute
# Remove from node 'foo' the taint with key 'dedicated' if one exists.
kubectl taint nodes foo dedicated-`
)

func NewCmdTaint(f *cmdutil.Factory, out io.Writer) *cobra.Command {
	// retrieve a list of handled resources from printer as valid args
	validArgs := []string{}
	p, err := f.Printer(nil, false, false, false, false, false, false, []string{})
	cmdutil.CheckErr(err)
	if p != nil {
		validArgs = p.HandledResources()
	}

	cmd := &cobra.Command{
		Use:     "taint NODE NAME KEY_1=VAL_1:TAINT_EFFECT_1 ... KEY_N=VAL_N:TAINT_EFFECT_N",
		Short:   "Update the taints on a node",
		Long:    fmt.Sprintf(taint_long, validation.LabelValueMaxLength),
		Example: taint_example,
		Run: func(cmd *cobra.Command, args []string) {
			err := RunTaint(f, out, cmd, args)
			cmdutil.CheckErr(err)
		},
		ValidArgs: validArgs,
	}
	cmd.Flags().Bool("overwrite", false, "If true, allow taints to be overwritten, otherwise reject taint updates that overwrite existing taints.")
	//cmd.Flags().StringP("selector", "l", "", "Selector (label query) to filter on")
	//cmd.Flags().Bool("all", false, "select all resources in the namespace of the specified resource types")
	//cmd.Flags().String("resource-version", "", "If non-empty, the labels update will only succeed if this is the current resource-version for the object. Only valid when specifying a single resource.")
	//kubectl.AddJsonFilenameFlag(cmd, &options.Filenames, "Filename, directory, or URL to a file identifying the resource to update the labels")
	//cmdutil.AddRecursiveFlag(cmd, &options.Recursive)
	cmd.Flags().Bool("dry-run", false, "If true, only print the object that would be sent, without sending it.")

	cmdutil.AddValidateFlags(cmd)
	cmdutil.AddPrinterFlags(cmd)
	cmdutil.AddInclude3rdPartyFlags(cmd)
	return cmd
}

func validateNoTaintOverwrites(node *api.Node, taints []api.Taint) error {
	allErrs := []error{}
	oldTaints, err := api.GetTaintsFromNodeAnnotations(node.Annotations)
	if err != nil {
		allErrs = append(allErrs, err)
		return utilerrors.NewAggregate(allErrs)
	}

	for _, taint := range taints {
		for _, oldeTaint := range oldTaints {
			if taint.Key == oldeTaint.Key {
				allErrs = append(allErrs, fmt.Errorf("Node '%s' already has a taint (%+v), and --overwrite is false", node.Name, taint))
			}
		}
	}
	return utilerrors.NewAggregate(allErrs)
}

func deleteTaintByKey(taints []api.Taint, key string) ([]api.Taint, error) {
	newTaints := []api.Taint{}
	found := false
	for _, taint := range taints {
		if taint.Key == key {
			found = true
			continue
		}
		newTaints = append(newTaints, taint)
	}

	if !found {
		return nil, fmt.Errorf("taint key=\"%s\" not found.", key)
	}
	return newTaints, nil
}

func reorganizeTaints(node *api.Node, overwrite bool, taints []api.Taint, remove []string) ([]api.Taint, error) {
	if !overwrite {
		if err := validateNoTaintOverwrites(node, taints); err != nil {
			return nil, err
		}
	}

	oldTaints, err := api.GetTaintsFromNodeAnnotations(node.Annotations)
	if err != nil {
		return nil, err
	}

	newTaints := append([]api.Taint{}, taints...)
	for _, oldTaint := range oldTaints {
		found := false
		for _, taint := range newTaints {
			if taint.Key == oldTaint.Key {
				found = true
				break
			}
		}
		if !found {
			newTaints = append(newTaints, oldTaint)
		}
	}

	allErrs := []error{}
	for _, taintToRemove := range remove {
		newTaints, err = deleteTaintByKey(newTaints, taintToRemove)
		if err != nil {
			allErrs = append(allErrs, err)
		}
	}

	if len(allErrs) > 0 {
		return nil, utilerrors.NewAggregate(allErrs)
	}

	return newTaints, nil
}

func parseTaints(spec []string) ([]api.Taint, []string, error) {
	var taints []api.Taint
	var remove []string
	for _, taintSpec := range spec {
		if strings.Index(taintSpec, "=") != -1 && strings.Index(taintSpec, ":") != -1 {
			parts := strings.Split(taintSpec, "=")
			if len(parts) != 2 || len(parts[1]) == 0 || !validation.IsQualifiedName(parts[0]) {
				return nil, nil, fmt.Errorf("invalid taint spec: %v", taintSpec)
			}

			parts2 := strings.Split(parts[1], ":")
			if len(parts2) != 2 || !validation.IsValidLabelValue(parts2[0]) {
				return nil, nil, fmt.Errorf("invalid taint spec: %v", taintSpec)
			}

			effect := api.TaintEffect(parts2[1])
			if effect != api.TaintEffectNoSchedule && effect != api.TaintEffectPreferNoSchedule {
				return nil, nil, fmt.Errorf("invalid taint spec: %v, unsupported taint effect", taintSpec)
			}

			newTaint := api.Taint{
				Key:    parts[0],
				Value:  parts2[0],
				Effect: effect,
			}

			taints = append(taints, newTaint)
		} else if strings.HasSuffix(taintSpec, "-") {
			remove = append(remove, taintSpec[:len(taintSpec)-1])
		} else {
			return nil, nil, fmt.Errorf("unknown taint spec: %v", taintSpec)
		}
	}
	return taints, remove, nil
}

func RunTaint(f *cmdutil.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	resources, taintArgs := []string{}, []string{}
	first := true
	for _, s := range args {
		isTaint := strings.Contains(s, "=") || strings.HasSuffix(s, "-")
		switch {
		case first && isTaint:
			first = false
			fallthrough
		case !first && isTaint:
			taintArgs = append(taintArgs, s)
		case first && !isTaint:
			resources = append(resources, s)
		case !first && !isTaint:
			return cmdutil.UsageError(cmd, "all resources must be specified before taint changes: %s", s)
		}
	}
	if len(resources) < 1 {
		return cmdutil.UsageError(cmd, "one or more resources must be specified as <resource> <name> or <resource>/<name>")
	}
	if len(taintArgs) < 1 {
		return cmdutil.UsageError(cmd, "at least one taint update is required")
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	targetNode, err := client.Nodes().Get(resources[1])
	if err != nil {
		return err
	}

	taints, remove, err := parseTaints(taintArgs)
	if err != nil {
		return cmdutil.UsageError(cmd, err.Error())
	}

	overwrite := cmdutil.GetFlagBool(cmd, "overwrite")
	if !overwrite {
		if err := validateNoTaintOverwrites(targetNode, taints); err != nil {
			return err
		}
	}

	newTaints, err := reorganizeTaints(targetNode, overwrite, taints, remove)
	if err != nil {
		return err
	}

	taintsData, err := json.Marshal(newTaints)
	if err != nil {
		return err
	}

	if targetNode.Annotations == nil {
		targetNode.Annotations = map[string]string{}
	}
	targetNode.Annotations[api.TaintsAnnotationKey] = string(taintsData)

	newNode, err := client.Nodes().Update(targetNode)
	if err != nil {
		return err
	}

	mapper, _ := f.Object(cmdutil.GetIncludeThirdPartyAPIs(cmd))

	message := "tainted"
	if len(targetNode.Annotations) != len(newNode.Annotations) ||
		targetNode.Annotations[api.TaintsAnnotationKey] != newNode.Annotations[api.TaintsAnnotationKey] {
		message = "taint failed"
	}

	gvk, err := api.Scheme.ObjectKind(targetNode)
	if err != nil {
		return err
	}
	_, res := meta.KindToResource(gvk)
	cmdutil.PrintSuccess(mapper, false, out, res.Resource, targetNode.Name, message)
	return nil
}
