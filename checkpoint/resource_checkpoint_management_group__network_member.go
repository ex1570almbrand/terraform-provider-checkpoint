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
			"members": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "Collection of Network objects identified by the name or UID.",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func createManagementGroupNetworkMember(d *schema.ResourceData, m interface{}) error {
	groupName := ""
	if v, ok := d.GetOk("name"); ok {
		groupName = v.(string)
	}
	members := make([]interface{}, 0)
	if val, ok := d.GetOk("members"); ok {
		members = val.(*schema.Set).List()
	}

	groupUid, err := createGroupAndNetworkObjectRelationship(groupName, members, m)
	if err == nil && len(groupUid) > 0 {
		dId := fmt.Sprintf("%s/network_object_members", groupUid)
		d.SetId(dId)
		return readManagementGroupNetworkMember(d, m)
	}
	if err == nil {
		return errors.New("Missing group UID or member UID in the response")
	}
	return err
}

func createGroupAndNetworkObjectRelationship(groupName string, members []interface{}, m interface{}) (string, error) {
	client := m.(*checkpoint.ApiClient)

	group := make(map[string]interface{})
	group["name"] = groupName
	group["members"] = members

	log.Println("Create Group Network Members - Map = ", group)

	addGroupMemberRes, err := client.ApiCall("set-group", group, client.GetSessionID(), true, false)
	if err != nil || !addGroupMemberRes.Success {
		if addGroupMemberRes.ErrorMsg != "" {
			return "", fmt.Errorf(addGroupMemberRes.ErrorMsg)
		}
		return "", fmt.Errorf(err.Error())
	}

	groupRes := addGroupMemberRes.GetData()
	groupUid := groupRes["uid"].(string)

	return groupUid, nil
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

	return nil
}

func updateManagementGroupNetworkMember(d *schema.ResourceData, m interface{}) error {
	if d.HasChange("name") { // This is basically a delete and create opretations, since the UID changes
		ids := strings.Split(d.Id(), "/")
		members := d.Get("members")
		deleteErr := deleteGroupAndNetworkObjectRelationship(ids[0], members.(*schema.Set).List(), m)
		if deleteErr != nil {
			d.SetId("")
			return deleteErr
		}

		groupName := ""
		if d.HasChange("name") {
			_, newName := d.GetChange("name")
			groupName = newName.(string)
		}
		_, newMembers := d.GetChange("members")
		groupUid, createErr := createGroupAndNetworkObjectRelationship(groupName, newMembers.(*schema.Set).List(), m)
		if createErr == nil && len(groupUid) > 0 {
			dId := fmt.Sprintf("%s/network_object_members", groupUid)
			d.SetId(dId)
			return readManagementGroupNetworkMember(d, m)
		}
		if createErr == nil {
			return errors.New("Missing group UID or member UID in the response")
		}
		return createErr

	} else if d.HasChange("members") { // This is a regular change
		client := m.(*checkpoint.ApiClient)
		group := make(map[string]interface{})

		group["name"] = d.Get("name")

		if ok := d.HasChange("members"); ok {
			if v, ok := d.GetOk("members"); ok {
				group["members"] = v.(*schema.Set).List()
			} else {
				oldMembers, _ := d.GetChange("members")
				group["members"] = map[string]interface{}{"remove": oldMembers.(*schema.Set).List()}
			}
		}

		log.Println("Update Group - Map = ", group)
		setGroupRes, _ := client.ApiCall("set-group", group, client.GetSessionID(), true, false)
		if !setGroupRes.Success {
			return fmt.Errorf(setGroupRes.ErrorMsg)
		}
	}

	return readManagementGroupNetworkMember(d, m)
}

func deleteManagementGroupNetworkMember(d *schema.ResourceData, m interface{}) error {
	ids := strings.Split(d.Id(), "/")
	members := d.Get("members")
	err := deleteGroupAndNetworkObjectRelationship(ids[0], members.(*schema.Set).List(), m)

	if err == nil {
		d.SetId("")
		return nil
	}

	return err
}

func deleteGroupAndNetworkObjectRelationship(groupUid string, members []interface{}, m interface{}) error {
	client := m.(*checkpoint.ApiClient)

	group := make(map[string]interface{})

	group["uid"] = groupUid
	group["members"] = map[string]interface{}{"remove": members}

	log.Println("Removing Group Network object relationship - Map = ", group)
	setGroupRes, _ := client.ApiCall("set-group", group, client.GetSessionID(), true, false)
	if !setGroupRes.Success {
		return fmt.Errorf(setGroupRes.ErrorMsg)
	}

	return nil
}
