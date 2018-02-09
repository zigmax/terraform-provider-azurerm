package azurerm

import (
	"fmt"
	"log"

  "github.com/Azure/azure-sdk-for-go/services/hdinsight/mgmt/2015-03-01-preview/hdinsight"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/response"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func resourceArmHDInsightCluster() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmHDInsightClusterCreate,
		Read:   resourceArmHDInsightClusterRead,
		Update: resourceArmHDInsightClusterUpdate,
		Delete: resourceArmHDInsightClusterDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
			},

			"location": locationSchema(),

			"resource_group_name": resourceGroupNameSchema(),

			"tags": tagsSchema(),

      "cluster_version": {
        Type: schema.TypeString,
        Required: true,
        ForceNew: true,
      },

      "os_type": {
        Type: schema.TypeString,
        Required: true,
        ForceNew: true,
        ValidateFunc: validation.StringInSlice([]string{
 					string(hdinsight.Linux),
 					string(hdinsight.Windows),
 				}, true),
      },

      "tier": {
        Type: schema.TypeString,
        Required: true,
        ForceNew: true,
        ValidateFunc: validation.StringInSlice([]string{
          string(hdinsight.Premium),
          string(hdinsight.Standard),
        }, true),
      },

      "cluster_definition": {
        Type: schema.TypeList
        Required: true,
        ForceNew: true,
        MaxItems: 1,
        Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
            "kind": {
              Type: schema.TypeString,
              Required: true,
              ForceNew: true,
            },
            "configurations": {
              Type:     schema.TypeMap,
              Required: true,
              ForceNew: true,
            },
          },
        },
      },
      "compute_profile": {
        Type: schema.TypeList
        Required: true,
        ForceNew: true,
        MaxItems: 1,
        Elem: &schema.Resource{
          Schema: map[string]*schema.Schema{
            "role": {
              Type: schema.TypeSet
              Required: true,
              ForceNew: true,
              Elem: &schema.Resource{
                Schema: map[string]*schema.Schema{
                  "name": {
                    Type: schema.TypeString,
                    Required: true,
                    ForceNew: true,
                  },
                  "target_instance_count": {
                    Type: schema.TypeInt,
                    Required: true,
                    ForceNew: true,
                  },
                  "hardware_profile": {
                    Type: schema.TypeList,
                    Required: true,
                    ForceNew: true,
                    MaxItems: 1,
                    Elem: &schema.Resource{
                      Schema: map[string]*schema.Schema{
                        "vm_size": {
													Type: schema.TypeString,
													Required: true,
													ForceNew: true,
												},
                      },
                    },
                  },
									"os_profile": {
										Type: schema.TypeList,
										Required: true,
										ForceNew: true,
										MaxItems: 1,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"username": {
													Type: schema.TypeString,
													Required: true,
													ForceNew: true,
												},
												"password": {
													Type: schema.TypeString,
													Optional: true,
													ForceNew: true,
												},
												"ssh_keys": {
													Type: schema.TypeList,
													Optional: true,
													ForceNew: true,
													Elem:     &schema.Schema{Type: schema.TypeString},
												},
											},
										},
									},
                },
              },
            },
          },
        },
      },
		},
	}
}


func resourceArmHDInsightClusterCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).clustersClient
	ctx := meta.(*ArmClient).StopContext

	name := d.Get("name").(string)
	resGroup := d.Get("resource_group_name").(string)
	location := d.Get("location").(string)

	tags := d.Get("tags").(map[string]interface{})
	metadata := expandTags(tags)

	clusterCreateProperties, err := expandClusterCreateProperties(d)
	if err != nil {
		return err
	}

	cluster := hdinsight.Cluster{
		Location: utils.String(location),
		Tags:     metadata,
		Properties: clusterCreateProperties,
	}

	future, err := client.Create(ctx, resGroup, name, cluster)
	if err != nil {
		return err
	}

	err = future.WaitForCompletion(ctx, client.Client)
	if err != nil {

		if response.WasConflict(future.Response()) {
			return fmt.Errorf("HDInsight Cluster name needs to be globally unique and %q is already in use.", name)
		}

		return err
	}

	resp, err := client.Get(ctx, resGroup, name)
	if err != nil {
		return err
	}

	d.SetId(*resp.ID)

	return resourceArmHDInsightClusterRead(d, meta)
}

func expandClusterCreateProperties(d *schema.ResourceData) (*hdinsight.ClusterCreateProperties, error) {
	clusterVersion := d.Get("cluster_version").(string)
	osType := hdinsight.OSType(d.Get("os_type").(string))
	tier := hdinsight.Tier(d.Get("tier").(string))
	clusterDefinition, err := expandClusterDefinition(d)
	if err != nil {
		return nil, err
	}
	computeProfile, err := expandComputeProfile(d)
	if err != nil {
		return nil, err
	}
}

func expandClusterDefinition(d *schema.ResourceData) (*hdinsight.ClusterDefinition, error) {
	clusterDefinitionConfig := d.Get("cluster_definition").([]interface{})
	clusterDef := clusterDefinitionConfig[0].(map[string]interface{})
	kind := clusterDef["kind"].(string)

	configurationMap := clusterDef["configurations"].(map[string]interface{})
	configurations := make(map[string]interface{}, len(configurationMap))
	for key, val := range configurationMap {
		configurations[key] = struct {
			Value interface{}
		}{
			Value: val,
		}
	}

	clusterDefinition := &hdinsight.ClusterDefinition{
		Kind: &kind,
		ClusterDefinition: &configurations
	}

	return clusterDefinition
}

func expandComputeProfile(d *schema.ResourceData) (*hdinsight.ComputeProfile, error) {
	computeProfileConfig := d.Get("compute_profile").([]interface{})
	computeProfileInfo := computeProfileConfig[0].(map[string]interface{})

	rolesConfig := computerProfileInfo["role"].(*schema.Set).List()
	roles := make([]hdinsight.Role, 0, len(rolesConfig))
	for _, role := range rolesConfig {
		roleConfig := role.(map[string]interface{})
		name := roleConfig["name"].(string)
		targetInstanceCount := roleConfig["target_instance_count"].(int)
		hardwareProfile := expandHardwareProfile(roleConfig)
		osProfile := expandOSProfile(roleConfig)
	}
}

func expandHardwareProfile(role map[string]interface{}) *hdinsight.HardwareProfile {
	hardwareProfileConfig := role["hardware_profile"].([]interface{})
	hardwareProfileInfo := hardwareProfileConfig[0].(map[string]interface{})
	vmSize := hardwareProfileInfo["vm_size"].(string)

	hardwareProfile := &hdinsight.HardwareProfile {
		VMSize: &vmSize,
	}

	return hardwareProfile
}

func expandOSProfile(role map[string]interface{}) *hdinsight.OsProfile {}

func resourceArmHDInsightClusterRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).clustersClient
	ctx := meta.(*ArmClient).StopContext

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}

	resGroup := id.ResourceGroup
	name := id.Path["clusters"]

	resp, err := client.Get(ctx, resGroup, name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			log.Printf("[INFO] Error reading HDInsight Cluster %q - removing from state", d.Id())
			d.SetId("")
			return nil
		}

		return fmt.Errorf("Error reading HDInsight Cluster %s: %v", name, err)
	}

	d.Set("name", name)
	d.Set("resource_group_name", resGroup)
	d.Set("location", azureRMNormalizeLocation(*resp.Location))

	flattenAndSetTags(d, resp.Tags)

	return nil
}

func resourceArmHDInsightClusterUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).clustersClient
	ctx := meta.(*ArmClient).StopContext

	name := d.Get("name").(string)
	resGroup := d.Get("resource_group_name").(string)

	metadata := expandTags(tags)

	parameters := hdinsight.ClusterPatchParameters{
		Tags:     metadata,
	}

	_, err := client.Update(ctx, resGroup, name, parameters)
	if err != nil {
		return err
	}

	resp, err := client.Get(ctx, resGroup, name)
	if err != nil {
		return err
	}

	d.SetId(*resp.ID)

	return resourceArmHDInsightClusterRead(d, meta)
}

func resourceArmHDInsightClusterDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).clustersClient
	ctx := meta.(*ArmClient).StopContext

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}

	resGroup := id.ResourceGroup
	name := id.Path["clusters"]

	future, err := client.Delete(ctx, resGroup, name)
	if err != nil {
		return fmt.Errorf("Error deleting HDInsight Cluster %s: %+v", name, err)
	}

	err = future.WaitForCompletion(ctx, client.Client)
	if err != nil {
		return err
	}

	return nil
}
