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
	groupName := ""
	if v, ok := d.GetOk("name"); ok {
		groupName = v.(string)
	}
	memberName := ""
	if v, ok := d.GetOk("member"); ok {
		memberName = v.(string)
	}

	groupUid, memberUid, err := createGroupAndNetworkObjectRelationship(groupName, memberName, m)
	if err == nil && len(groupUid) > 0 && len(memberUid) > 0 {
		dId := fmt.Sprintf("%s/%s", groupUid, memberUid)
		d.SetId(dId)
		return readManagementGroupNetworkMember(d, m)
	}
	if err == nil {
		return errors.New("Missing group UID or member UID in the response")
	}
	return err
}

func createGroupAndNetworkObjectRelationship(groupName string, noName string, m interface{}) (string, string, error) {
	client := m.(*checkpoint.ApiClient)

	group := make(map[string]interface{})
	group["name"] = groupName
	group["members"] = noName

	log.Println("Create Group Network Member - Map = ", group)

	addGroupMemberRes, err := client.ApiCall("set-group", group, client.GetSessionID(), true, false)
	if err != nil || !addGroupMemberRes.Success {
		if addGroupMemberRes.ErrorMsg != "" {
			return "", "", fmt.Errorf(addGroupMemberRes.ErrorMsg)
		}
		return "", "", fmt.Errorf(err.Error())
	}

	groupRes := addGroupMemberRes.GetData()
	membersJson := groupRes["members"].([]interface{})
	if membersJson == nil && len(membersJson) == 0 {
		return "", "", errors.New("No members in the set-group response")
	}
	memberUid := ""
	if len(membersJson) > 0 {
		for _, member := range membersJson {
			member := member.(map[string]interface{})
			if member["name"].(string) == noName {
				// Get the correct member UID as the response may contain other existing members as well
				memberUid = member["uid"].(string)
			}
		}
	}
	if memberUid == "" {
		return "", "", errors.New("New member not found in the set-group response")
	}
	groupUid := groupRes["uid"].(string)

	return groupUid, memberUid, nil
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
	if d.HasChange("name") || d.HasChange("member") {
		// Delete the current relationship if succeful set dId to "" -> So that state is updated with real world
		ids := strings.Split(d.Id(), "/")
		deleteErr := deleteGroupAndNetworkObjectRelationship(ids[0], ids[1], m)
		if deleteErr == nil {
			d.SetId("")
			return deleteErr
		}
		// Create a new relationship and if successful set dId to new "<groupUID>/<networkObjectID>"
		groupName := getCurrentOrChangedValue(d, "name")
		memberName := getCurrentOrChangedValue(d, "member")
		groupUid, memberUid, createErr := createGroupAndNetworkObjectRelationship(groupName, memberName, m)
		if createErr == nil && len(groupUid) > 0 && len(memberUid) > 0 {
			dId := fmt.Sprintf("%s/%s", groupUid, memberUid)
			d.SetId(dId)
			return readManagementGroupNetworkMember(d, m)
		}
		if createErr == nil {
			return errors.New("Missing group UID or member UID in the response")
		}
		return createErr
	}
	// Do nothing if no changes are detected
	return readManagementGroupNetworkMember(d, m)
}

func getCurrentOrChangedValue(d *schema.ResourceData, paramName string) string {
	if d.HasChange(paramName) {
		_, newName := d.GetChange(paramName)
		return newName.(string)
	} else {
		return d.Get("name").(string)
	}
}

func deleteManagementGroupNetworkMember(d *schema.ResourceData, m interface{}) error {
	ids := strings.Split(d.Id(), "/")

	group := make(map[string]interface{})

	group["uid"] = ids[0]
	group["members"] = map[string]interface{}{"remove": ids[1]}

	err := deleteGroupAndNetworkObjectRelationship(ids[0], ids[1], m)
	if err == nil {
		d.SetId("")
		return nil
	}

	return err
}

func deleteGroupAndNetworkObjectRelationship(groupUid string, noUid string, m interface{}) error {
	client := m.(*checkpoint.ApiClient)

	group := make(map[string]interface{})

	group["uid"] = groupUid
	group["members"] = map[string]interface{}{"remove": noUid}

	log.Println("Removing Group Network object relationship - Map = ", group)
	setGroupRes, _ := client.ApiCall("set-group", group, client.GetSessionID(), true, false)
	if !setGroupRes.Success {
		return fmt.Errorf(setGroupRes.ErrorMsg)
	}

	return nil
}
