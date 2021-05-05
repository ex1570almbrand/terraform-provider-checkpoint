package checkpoint

import (
	"errors"
	"fmt"
	checkpoint "github.com/CheckPointSW/cp-mgmt-api-go-sdk/APIFiles"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"log"
	"strings"
)

func resourceManagementGroupNetworkMember() *schema.Resource {
	return &schema.Resource{
		Create: createManagementGroupNetworkMember,
		Read:   readManagementGroupNetworkMember,
		Update: updateManagementGroupNetworkMember,
		Delete: deleteManagementGroupNetworkMember,
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
				Description: "Network object name. The object to be added to a group",
			},
		},
	}
}

func createManagementGroupNetworkMember(d *schema.ResourceData, m interface{}) error {
	client := m.(*checkpoint.ApiClient)

	group := make(map[string]interface{})

	if v, ok := d.GetOk("name"); ok {
		group["name"] = v.(string)
	}
	newMemberName := ""
	if v, ok := d.GetOk("member"); ok {
		group["members"] = v.(string)
		newMemberName = v.(string)
	}

	log.Println("Create Group Network Member - Map = ", group)

	addGroupMemberRes, err := client.ApiCall("set-group", group, client.GetSessionID(), true, false)
	if err != nil || !addGroupMemberRes.Success {
		if addGroupMemberRes.ErrorMsg != "" {
			return fmt.Errorf(addGroupMemberRes.ErrorMsg)
		}
		return fmt.Errorf(err.Error())
	}

	groupRes := addGroupMemberRes.GetData()
	membersJson := groupRes["members"].([]interface{})
	if membersJson == nil && len(membersJson) == 0 {
		return errors.New("No members in the set-group response")
	}
	memberUid := ""
	if len(membersJson) > 0 {
		for _, member := range membersJson {
			member := member.(map[string]interface{})
			if member["name"].(string) == newMemberName {
				// Get the correct member UID as the response may contain other existing members as well
				memberUid = member["uid"].(string)
			}
		}
	}
	if memberUid == "" {
		return errors.New("New member not found in the set-group response")
	}
	groupUid := groupRes["uid"].(string)

	dId := fmt.Sprintf("%s/%s", groupUid, memberUid)

	d.SetId(dId)

	return readManagementGroupNetworkMember(d, m)
}

func readManagementGroupNetworkMember(d *schema.ResourceData, m interface{}) error {

	client := m.(*checkpoint.ApiClient)

	ids := strings.Split(d.Id(), "/")

	payload := map[string]interface{}{
		"uid": ids[0], // The first is the group object UID
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

	group := showGroupRes.GetData()

	if v := group["name"]; v != nil {
		_ = d.Set("name", v)
	}

	if group["members"] != nil {
		membersJson := group["members"].([]interface{})
		memberName := ""
		if len(membersJson) > 0 {
			for _, member := range membersJson {
				member := member.(map[string]interface{})
				if member["uid"].(string) == ids[1] {
					// Set "member" to member name if and only if is part of the members array returned by the API
					memberName = member["name"].(string)
				}
			}
		}
		_ = d.Set("member", memberName)
	} else {
		_ = d.Set("member", nil)
	}

	return nil
}

func updateManagementGroupNetworkMember(d *schema.ResourceData, m interface{}) error {

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

func deleteManagementGroupNetworkMember(d *schema.ResourceData, m interface{}) error {
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
