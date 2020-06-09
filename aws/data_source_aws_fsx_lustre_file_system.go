package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/fsx"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func dataSourceAwsFsxLustreFileSystem() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceAwsFsxLustreFileSystemRead,

		Schema: map[string]*schema.Schema{
			"id": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"filter": dataSourceFiltersSchema(),
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"dns_name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"export_path": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"import_path": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"imported_file_chunk_size": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"network_interface_ids": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"owner_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"mount_name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"security_group_ids": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"storage_capacity": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"subnet_ids": {
				Type:     schema.TypeSet,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Computed: true,
			},
			"tags": tagsSchema(),
			"vpc_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"weekly_maintenance_start_time": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"deployment_type": {
				Type:  schema.TypeString,
				Computed: true,
			},
			"per_unit_storage_throughput": {
				Type: schema.TypeInt,
				Computed: true,
			},
		},
	}
}

func dataSourceAwsFsxLustreFileSystemRead(d *schema.ResourceData, meta interface{}) error {
	// If we can get the data source via id, do it.
	if _, ok := d.GetOk("id"); ok {
		d.SetId(d.Get("id").(string))
		return resourceAwsFsxLustreFileSystemRead(d, meta)
	}
	// Otherwise, try to find the fsx file system via filters.
	// The AWS golang API does not yet support filter queries on fsx file systems. We could just create a special
	// `name_tag` parameter to use, but that would need to be changed whenever HashiCorp actually releases this feature.
	// Instead, we maintain the `filter` api that terraform will eventually use for this feature. Instead of passing
	// that filter to the golang api which doesn't support it yet (https://docs.aws.amazon.com/sdk-for-go/api/service/fsx/#DescribeFileSystemsInput),
	// we unpack the name tag from the filter, request all fsx file systems, then return the one that matches that tag.

	filters, ok := d.GetOk("filter")
	if !ok {
		return fmt.Errorf("If the 'id' field is not provided, a 'filter' block must be provided.")
	}
	// Convert the filter object into a list of filters.
	filtersList := filters.(*schema.Set).List()
	if len(filtersList) > 1 {
		return fmt.Errorf("For now, only supports a single filter.")
	}
	// After verifying there is only 1 filter, unpack it as a map.
	firstFilter := filtersList[0].(map[string]interface{})
	// Verify that the filter has 'tag:Name' as its name.
	filterName := firstFilter["name"]
	if filterName != "tag:Name" {
		return fmt.Errorf("For now, only supports the 'tag:Name' filter.")
	}
	// Verify there is only one value, and unpack it as the nameTag.
	filterValues := firstFilter["values"].([]interface{})
	if len(filterValues) != 1 {
		return fmt.Errorf("The 'tag:Name' filter must have 1 value.")
	}
	nameTag := filterValues[0].(string)
	// List all the filesystems.
	conn := meta.(*AWSClient).fsxconn
	resp, err := conn.DescribeFileSystems(&fsx.DescribeFileSystemsInput{})
	if err != nil {
		return err
	}
	// Iterate through all filesystems, and if their Name tag matches our requested name tag,
	// append that id to a list.
	var ids []string
	for _, fs := range resp.FileSystems {
		for _, t:= range fs.Tags {
			if *t.Value == nameTag && *t.Key == "Name" {
				ids = append(ids, *fs.FileSystemId)
			}
		}
	}
	if len(ids) > 1 {
		return fmt.Errorf("Found multiple file systems with `Name` tag %q, don't know which one to return: %v", nameTag, ids)
	} else if len(ids) == 1 {
		// If we have a single id to use, use that ID to find information about the entire filesystem and populate
		// it properly as a terraform data resource.
		d.SetId(ids[0])
		return resourceAwsFsxLustreFileSystemRead(d, meta)
	}
	return fmt.Errorf("Found no matching file systems, specify either an `id` or a valid `name_tag`.")
}
