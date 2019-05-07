package render

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/openshift/cluster-kube-scheduler-operator/pkg/operator/v410_00_assets"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"

	genericrender "github.com/openshift/library-go/pkg/operator/render"
	genericrenderoptions "github.com/openshift/library-go/pkg/operator/render/options"
)

const (
	bootstrapVersion = "v4.1.0"
)

// renderOpts holds values to drive the render command.
type renderOpts struct {
	manifest genericrenderoptions.ManifestOptions
	generic  genericrenderoptions.GenericOptions
}

// NewRenderCommand creates a render command.
func NewRenderCommand() *cobra.Command {
	renderOpts := renderOpts{
		generic:  *genericrenderoptions.NewGenericOptions(),
		manifest: *genericrenderoptions.NewManifestOptions("kube-scheduler", "openshift/origin-hyperkube:latest"),
	}
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render kube-scheduler bootstrap manifests, secrets and configMaps",
		Run: func(cmd *cobra.Command, args []string) {
			if err := renderOpts.Validate(); err != nil {
				klog.Fatal(err)
			}
			if err := renderOpts.Run(); err != nil {
				klog.Fatal(err)
			}
		},
	}

	renderOpts.AddFlags(cmd.Flags())

	return cmd
}

func (r *renderOpts) AddFlags(fs *pflag.FlagSet) {
	r.manifest.AddFlags(fs, "scheduler")
	r.generic.AddFlags(fs, schema.GroupVersionKind{Group: "componentconfig", Version: "v1alpha1", Kind: "KubeSchedulerConfiguration"})
}

// Validate verifies the inputs.
func (r *renderOpts) Validate() error {
	if err := r.manifest.Validate(); err != nil {
		return err
	}
	if err := r.generic.Validate(); err != nil {
		return err
	}

	return nil
}

// Complete fills in missing values before command execution.
func (r *renderOpts) Complete() error {
	if err := r.manifest.Complete(); err != nil {
		return err
	}
	if err := r.generic.Complete(); err != nil {
		return err
	}
	return nil
}

type TemplateData struct {
	genericrenderoptions.ManifestConfig
	genericrenderoptions.FileConfig
}

// Run contains the logic of the render command.
func (r *renderOpts) Run() error {
	if err := r.Complete(); err != nil {
		return err
	}

	renderConfig := TemplateData{}
	if err := r.manifest.ApplyTo(&renderConfig.ManifestConfig); err != nil {
		return err
	}
	if err := r.generic.ApplyTo(
		&renderConfig.FileConfig,
		genericrenderoptions.Template{FileName: "defaultconfig.yaml", Content: v410_00_assets.MustAsset(filepath.Join(bootstrapVersion, "kube-scheduler", "defaultconfig.yaml"))},
		mustReadTemplateFile(filepath.Join(r.generic.TemplatesDir, "config", "bootstrap-config-overrides.yaml")),
		mustReadTemplateFile(filepath.Join(r.generic.TemplatesDir, "config", "config-overrides.yaml")),
		&renderConfig,
		nil,
	); err != nil {
		return err
	}

	// add additional kubeconfig asset
	if kubeConfig, err := r.readBootstrapSecretsKubeconfig(); err != nil {
		return fmt.Errorf("failed to read %s/kubeconfig: %v", r.manifest.SecretsHostPath, err)
	} else {
		renderConfig.Assets["kubeconfig"] = kubeConfig
	}

	return genericrender.WriteFiles(&r.generic, &renderConfig.FileConfig, renderConfig)
}

func (r *renderOpts) readBootstrapSecretsKubeconfig() ([]byte, error) {
	return ioutil.ReadFile(filepath.Join(r.generic.AssetInputDir, "..", "auth", "kubeconfig"))
}

func mustReadTemplateFile(fname string) genericrenderoptions.Template {
	bs, err := ioutil.ReadFile(fname)
	if err != nil {
		panic(fmt.Sprintf("Failed to load %q: %v", fname, err))
	}
	return genericrenderoptions.Template{FileName: fname, Content: bs}
}
