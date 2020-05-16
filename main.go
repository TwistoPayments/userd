package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
)

type gooleUsersConfig struct {
	Groups map[string][]string
}
type serverConfig struct {
	SystemUsers []localUser
	GoogleUsers gooleUsersConfig
}

type localUser struct {
	Username        string
	FullName        string
	Email           string
	CanHavePassword bool
	SSHKeys         []string
	groups          map[string]struct{} // set of Linux groups
	SSHKeysUrl      string
	Home            string
	UID             int
	GID             int
}

func (u *localUser) getGroups() []string {
	list := make([]string, 0, len(u.groups))
	for g := range u.groups {
		list = append(list, g)
	}
	return list
}

func (u *localUser) addGroup(group string) {
	matched, err := regexp.Match("^[a-zA-Z]+$", []byte(group))
	if err != nil {
		panic(err)
	}
	if !matched {
		log.Fatalf("Invalid group name: %s", group)
	}
	if u.groups == nil {
		u.groups = make(map[string]struct{})
	}
	u.groups[group] = struct{}{}
}

func (u *localUser) gecosString() string {
	if u.Email != "" && u.FullName != "" {
		return fmt.Sprintf("%s <%s>,,,", strings.ReplaceAll(u.FullName, ",", "_"), u.Email)
	} else if u.FullName != "" {
		return u.FullName
	} else if u.Email != "" {
		return u.Email
	} else {
		return ""
	}
}

func syncUsers(usersFromDirectory map[string]localUser) error {
	existingUsers, err := getExistingUsers()
	if err != nil {
		return err
	}

	for _, u := range usersFromDirectory {
		existingUser, present := existingUsers[u.Username]
		if !present {
			log.Printf("INFO: Adding user %#v", u)
			existingUser, err = addUser(u.Username, u.gecosString(), u.Home)
			if err != nil {
				return err
			}
		}

		u.Home = existingUser.Home
		u.GID = existingUser.GID
		u.UID = existingUser.UID

		err = ensureGroups(existingUser, u.getGroups())
		if err != nil {
			return fmt.Errorf("Error setting up groups for %s:\n%w", u.Username, err)
		}

		//err = writeUserInfo(existingUsername, userInfo{existingUsername.Email})
		//if err != nil {
		//	return fmt.Errorf("Error writing %s's user info:\n%w", existingUsername.Username, err)
		//}

		if !u.CanHavePassword {
			err := ensureNoPassword(u.Username)
			if err != nil {
				return err
			}
		}

		err = syncKeys(u)
		if err != nil {
			return err
		}
	}

	for existingUsername, existingUser := range existingUsers {
		_, present := usersFromDirectory[existingUsername]
		if !present {
			if existingUser.UID < 1000 {
				err := checkNoPassword(existingUsername)
				if err != nil {
					return err
				}
				err = checkNoAuthorizedKeys(existingUser)
				if err != nil {
					return err
				}
				log.Printf("DEBUG: Keeping user %v", existingUser)
			} else {
				log.Printf("INFO: Removing user %v", existingUser)
				err = removeUser(existingUsername)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func syncKeys(u localUser) error {
	authorizedKeys := new(bytes.Buffer)

	if u.SSHKeysUrl != "" {
		resp, err := http.Get(u.SSHKeysUrl)
		if err != nil {
			return fmt.Errorf("Error getting keys for %s from %s:\n%w", u.Username, u.SSHKeysUrl, err)
		} else {
			defer resp.Body.Close()
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return fmt.Errorf("Error getting keys for %s from %s:\n%s", u.Username, u.SSHKeysUrl, resp.Status)
		}

		authorizedKeys.Write([]byte("# Public keys from "))
		authorizedKeys.Write([]byte(u.SSHKeysUrl))
		authorizedKeys.Write([]byte("\n"))

		_, err = authorizedKeys.ReadFrom(resp.Body)
		if err != nil {
			return fmt.Errorf("Error reading keys for %s from %s:\n%w", u.Username, u.SSHKeysUrl, err)
		}
		authorizedKeys.Write([]byte("\n"))
	}

	if u.SSHKeys != nil {
		for _, key := range u.SSHKeys {
			authorizedKeys.Write(bytes.TrimSpace([]byte(key)))
			authorizedKeys.Write([]byte("\n"))
		}
	}

	if u.Home == "" {
		panic(fmt.Errorf("unexpected empty home for %s", u.Username))
	}

	return setAuthorizedKeys(u, authorizedKeys)
}

/*
func writeUserInfo(u localUser, ui userInfo) error {
	infoFileName := filepath.Join("/home/", u.Username, ".userd-info.json")

	jsonBlob, err := json.MarshalIndent(ui, "", "  ")
	if err != nil {
		return err
	}

	return ensureFileContent(infoFileName, jsonBlob)
}
*/

func includeSystemUsers(directoryUsers map[string]localUser, systemUsers []localUser) {
	for _, u := range systemUsers {
		directoryUser, present := directoryUsers[u.Username]
		if !present {
			directoryUsers[u.Username] = u
		} else {
			log.Fatalf(
				"User %s defined both in directory and in systemUsers.\nDirectory definition: %#v\n",
				u.Username, directoryUser,
			)
		}
	}
}

func main() {
	sc := serverConfig{
		SystemUsers: []localUser{
			{Username: "root", CanHavePassword: false, SSHKeys: []string{}},
			{Username: "ubuntu", CanHavePassword: false, SSHKeys: nil},
		},
		GoogleUsers: gooleUsersConfig{
			Groups: map[string][]string{
				"devops@twisto.cz": {"sudo", "dev"},
			},
		},
	}

	directoryUsers, err := getDirectoryUsers(sc.GoogleUsers)
	if err != nil {
		log.Fatalf("Can't get directory users: %v", err)
	}
	includeSystemUsers(directoryUsers, sc.SystemUsers)

	log.Printf("DEBUG: %#v\n", sc)
	log.Printf("DEBUG: %#v\n", directoryUsers)
	/*
		directoryUsers := map[string]localUser{
			"testik: {
				Username: "testik",
				FullName: "Testík, Testovič",
				Email:    "test@example.com",
				groups:   map[string]struct{}{"sudo": struct{}{}},
			},
			"dudek": {
				Username: "dudek",
				FullName: "Dudy",
				Email:    "dudy@example.com",
				groups:   map[string]struct{}{"asudo": struct{}{}},
			},
		}
	*/
	err = syncUsers(directoryUsers)
	if err != nil {
		log.Fatalf("Error creating user: %v", err)
	}
}
