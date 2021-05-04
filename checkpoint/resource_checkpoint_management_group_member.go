package checkpoint

import (
	"errors"
	"fmt"
	checkpoint "github.com/CheckPointSW/cp-mgmt-api-go-sdk/APIFiles"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"log"
)

func resourceManagementGroupMember() *schema.Resource {
	return &schema.Resource{
		Create: createManagementGroupMember,
		Read:   readManagementGroupMember,
		Update: updateManagementGroupMember,
		Delete: deleteManagementGroupMember,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Existing group object name. Identifies the group to add members to",
			},
			"member": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Object name. The object to be added to a group",
			},
		},
	}
}

func createManagementGroupMember(d *schema.ResourceData, m interface{}) error {
	client := m.(*checkpoint.ApiClient)

	group := make(map[string]interface{})

	if v, ok := d.GetOk("name"); ok {
		group["name"] = v.(string)
	}
	if v, ok := d.GetOk("member"); ok {
		group["members"] = v.(string)
	}

	log.Println("Create Group Member - Map = ", group)

	addGroupMemberRes, err := client.ApiCall("set-group", group, client.GetSessionID(), true, false)
	if err != nil || !addGroupMemberRes.Success {
		if addGroupMemberRes.ErrorMsg != "" {
			return fmt.Errorf(addGroupMemberRes.ErrorMsg)
		}
		return fmt.Errorf(err.Error())
	}

	groupRes := addGroupMemberRes.GetData()
	membersJson := groupRes["members"].([]interface{})
	if membersJson == nil && len(membersJson) > 0 {
		return errors.New("No members in the set-group response")
	}
	firstMember := membersJson[0].(map[string]interface{})
	groupUid := groupRes["uid"].(string)

	dId := fmt.Sprintf("%s/%s", groupUid, firstMember["uid"].(string))

	d.SetId(dId)

	return readManagementGroup(d, m)
}

func readManagementGroupMember(d *schema.ResourceData, m interface{}) error {

	client := m.(*checkpoint.ApiClient)

	payload := map[string]interface{}{
		"uid": d.Id(), // TODO: split on "/" and take the first string
	}

	showGroupRes, err := client.ApiCall("show-group", payload, client.GetSessionID(), true, false)
	if err != nil {
		return fmt.Errorf(err.Error())
	}
	if !showGroupRes.Success {
		// Handle delete resource from other clients
		if objectNotFound(showGroupRes.GetData()["code"].(string)) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf(showGroupRes.ErrorMsg)
	}

	// TODO: Repeat the above for network member (or any member actuall)
	// Problem: How do we get any object? Or we should change the names and code to only work with network objects

	group := showGroupRes.GetData()

	if v := group["name"]; v != nil {
		_ = d.Set("name", v)
	}

	if v := group["comments"]; v != nil {
		_ = d.Set("comments", v)
	}

	if v := group["color"]; v != nil {
		_ = d.Set("color", v)
	}

	if group["members"] != nil {
		membersJson := group["members"].([]interface{})
		membersIds := make([]string, 0)
		if len(membersJson) > 0 {
			// Create slice of members names
			for _, member := range membersJson {
				member := member.(map[string]interface{})
				membersIds = append(membersIds, member["name"].(string))
			}
		}
		_ = d.Set("members", membersIds)
	} else {
		_ = d.Set("members", nil)
	}

	if group["tags"] != nil {
		tagsJson := group["tags"].([]interface{})
		var tagsIds = make([]string, 0)
		if len(tagsJson) > 0 {
			// Create slice of tag names
			for _, tag := range tagsJson {
				tag := tag.(map[string]interface{})
				tagsIds = append(tagsIds, tag["name"].(string))
			}
		}
		_ = d.Set("tags", tagsIds)
	} else {
		_ = d.Set("tags", nil)
	}

	return nil
}

func updateManagementGroupMember(d *schema.ResourceData, m interface{}) error {

	client := m.(*checkpoint.ApiClient)
	group := make(map[string]interface{})

	if d.HasChange("name") {
		oldName, newName := d.GetChange("name")
		group["name"] = oldName.(string)
		group["new-name"] = newName.(string)
	} else {
		group["name"] = d.Get("name")
	}

	if ok := d.HasChange("members"); ok {
		if v, ok := d.GetOk("members"); ok {
			group["members"] = v.(*schema.Set).List()
		} else {
			oldMembers, _ := d.GetChange("members")
			group["members"] = map[string]interface{}{"remove": oldMembers.(*schema.Set).List()}
		}
	}
	if ok := d.HasChange("tags"); ok {
		if v, ok := d.GetOk("tags"); ok {
			group["tags"] = v.(*schema.Set).List()
		} else {
			oldTags, _ := d.GetChange("tags")
			group["tags"] = map[string]interface{}{"remove": oldTags.(*schema.Set).List()}
		}
	}

	if ok := d.HasChange("comments"); ok {
		group["comments"] = d.Get("comments")
	}
	if ok := d.HasChange("color"); ok {
		group["color"] = d.Get("color")
	}

	if v, ok := d.GetOkExists("ignore_errors"); ok {
		group["ignore-errors"] = v.(bool)
	}
	if v, ok := d.GetOkExists("ignore_warnings"); ok {
		group["ignore-warnings"] = v.(bool)
	}

	log.Println("Update Group - Map = ", group)
	setGroupRes, _ := client.ApiCall("set-group", group, client.GetSessionID(), true, false)
	if !setGroupRes.Success {
		return fmt.Errorf(setGroupRes.ErrorMsg)
	}

	return readManagementGroup(d, m)
}

func deleteManagementGroupMember(d *schema.ResourceData, m interface{}) error {
	client := m.(*checkpoint.ApiClient)
	payload := map[string]interface{}{
		"uid": d.Id(),
	}
	deleteGroupRes, _ := client.ApiCall("delete-group", payload, client.GetSessionID(), true, false)
	if !deleteGroupRes.Success {
		return fmt.Errorf(deleteGroupRes.ErrorMsg)
	}
	d.SetId("")

	return nil
}
