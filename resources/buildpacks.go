package resources

import (
	"github.com/hashicorp/terraform/helper/schema"
	"code.cloudfoundry.org/cli/cf/models"
	"github.com/orange-cloudfoundry/terraform-provider-cloudfoundry/cf_client"
	"strings"
	"log"
	"path"
	"github.com/orange-cloudfoundry/terraform-provider-cloudfoundry/resources/caching"
	"path/filepath"
	"os"
)

type CfBuildpackResource struct {
	CfResource
}

func NewCfBuildpackResource() CfResource {
	return &CfBuildpackResource{}
}
func (c CfBuildpackResource) resourceObject(d *schema.ResourceData) (models.Buildpack, error) {
	var err error
	position := d.Get("position").(int)
	enabled := d.Get("enabled").(bool)
	locked := d.Get("locked").(bool)
	filename := d.Get("path").(string)
	if filename != "" {
		filename, err = c.generateFilename(d.Get("path").(string))
		if err != nil {
			return models.Buildpack{}, err
		}
	}

	return models.Buildpack{
		GUID: d.Id(),
		Name: d.Get("name").(string),
		Enabled: &enabled,
		Locked: &locked,
		Position: &position,
		Filename: filename,
	}, nil
}
func (c CfBuildpackResource) Create(d *schema.ResourceData, meta interface{}) error {
	client := meta.(cf_client.Client)
	buildpack, err := c.resourceObject(d)
	if err != nil {
		return err
	}
	var buildpackCf models.Buildpack
	if ok, _ := c.Exists(d, meta); ok {
		log.Printf(
			"[INFO] skipping creation of buildpack %s/%s because it already exists on your Cloud Foundry",
			client.Config().ApiEndpoint,
			buildpack.Name,
		)
		buildpackCf, err = c.getBuildpackFromCf(client, d.Id(), false)
		if err != nil {
			return err
		}
	} else {
		buildpackCf, err = client.Buildpack().Create(buildpack.Name, buildpack.Position, buildpack.Enabled, buildpack.Locked)
		if err != nil {
			return err
		}
		c.Exists(d, meta)
	}
	buildpack.GUID = d.Id()
	if c.isSystemBuildpackManaged(buildpack) {
		return nil
	}
	return c.updateBuildpack(client, buildpackCf, buildpack, d.Get("path").(string))
}

func (c CfBuildpackResource) Exists(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := meta.(cf_client.Client)
	name := d.Get("name").(string)
	buildpack, err := client.Buildpack().FindByName(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}
	d.SetId(buildpack.GUID)
	return true, nil
}
func (c CfBuildpackResource) generateFilename(buildpackPath string) (string, error) {
	if buildpackPath == "" {
		return "", nil
	}
	if (IsWebURL(buildpackPath)) {
		return path.Base(buildpackPath), nil
	}
	buildpackFileName := filepath.Base(buildpackPath)
	dir, err := filepath.Abs(buildpackPath)
	if err != nil {
		return "", err
	}
	buildpackFileName = filepath.Base(dir)
	stats, err := os.Stat(dir)
	if err != nil {
		return "", err
	}

	if stats.IsDir() {
		buildpackFileName += ".zip" // FIXME: remove once #71167394 is fixed
	}
	return buildpackFileName, nil
}
func (c CfBuildpackResource) Read(d *schema.ResourceData, meta interface{}) error {
	client := meta.(cf_client.Client)
	name := d.Get("name").(string)
	buildpack, err := c.getBuildpackFromCf(client, d.Id(), true)
	if err != nil {
		return err
	}
	if buildpack.GUID == "" {
		log.Printf(
			"[WARN] removing buildpack %s/%s from state because it no longer exists in your Cloud Foundry",
			client.Config().ApiEndpoint,
			name,
		)
		d.SetId("")
		return nil
	}
	d.Set("name", buildpack.Name)
	bp, err := c.resourceObject(d)
	if err != nil {
		return err
	}
	if c.isSystemBuildpackManaged(bp) {
		return nil
	}
	if bp.Filename != buildpack.Filename && d.Get("path").(string) != "" {
		d.Set("path", buildpack.Filename)
	}
	d.Set("position", *buildpack.Position)
	d.Set("enabled", *buildpack.Enabled)
	d.Set("locked", *buildpack.Locked)
	return nil

}
func (c CfBuildpackResource) isSystemBuildpackManaged(buildpack models.Buildpack) bool {
	if buildpack.Filename == "" && *buildpack.Position == 1 && *buildpack.Enabled == true && *buildpack.Locked == false {
		return true
	}
	return false
}
func (c CfBuildpackResource) Update(d *schema.ResourceData, meta interface{}) error {
	client := meta.(cf_client.Client)
	name := d.Get("name").(string)
	buildpack, err := c.resourceObject(d)
	if err != nil {
		return err
	}
	buildpackCf, err := c.getBuildpackFromCf(client, d.Id(), false)
	if err != nil {
		return err
	}
	if buildpackCf.GUID == "" {
		log.Printf(
			"[WARN] removing organization %s/%s from state because it no longer exists in your Cloud Foundry",
			client.Config().ApiEndpoint,
			name,
		)
		d.SetId("")
		return nil
	}
	return c.updateBuildpack(client, buildpackCf, buildpack, d.Get("path").(string))
}
func (c CfBuildpackResource) updateBuildpack(client cf_client.Client, buildpackFrom, buildpackTo models.Buildpack, buildpackPath string) error {
	var err error
	if (buildpackTo.Locked != buildpackFrom.Locked ||
		buildpackTo.Enabled != buildpackFrom.Enabled ||
		buildpackTo.Name != buildpackFrom.Name ||
		buildpackTo.Position != buildpackFrom.Position) {

		_, err = client.Buildpack().Update(buildpackTo)
		if err != nil {
			return err
		}
	}
	if buildpackTo.Filename == "" {
		return nil
	}
	if buildpackTo.Filename != buildpackFrom.Filename {
		file, _, err := client.BuildpackBits().CreateBuildpackZipFile(buildpackPath)
		if err != nil {
			return err
		}
		err = client.BuildpackBits().UploadBuildpack(buildpackTo, file, buildpackTo.Filename)
		if err != nil {
			return err
		}
	}
	return nil
}
func (c CfBuildpackResource) Delete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(cf_client.Client)
	bp, err := c.resourceObject(d)
	if err != nil {
		return err
	}
	if c.isSystemBuildpackManaged(bp) {
		return nil
	}
	return client.Buildpack().Delete(d.Id())
}
func (c CfBuildpackResource) getBuildpackFromCf(client cf_client.Client, bpGuid string, updateCache bool) (models.Buildpack, error) {
	buildpacks, err := caching.GetBuildpacks(client, updateCache)
	if err != nil {
		return models.Buildpack{}, err
	}
	for _, buildpack := range buildpacks {
		if buildpack.GUID == bpGuid {
			return buildpack, nil
		}
	}
	return models.Buildpack{}, nil
}
func (c CfBuildpackResource) Schema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": &schema.Schema{
			Type:     schema.TypeString,
			Required: true,
			ForceNew: true,
		},
		"path": &schema.Schema{
			Type:     schema.TypeString,
			Optional: true,
		},
		"position": &schema.Schema{
			Type:     schema.TypeInt,
			Optional: true,
			Default: 1,
		},
		"enabled": &schema.Schema{
			Type:     schema.TypeBool,
			Optional: true,
			Default: true,
		},
		"locked": &schema.Schema{
			Type:     schema.TypeBool,
			Optional: true,
		},
	}
}

