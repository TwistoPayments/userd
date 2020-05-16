package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//PasswdEntry represents a line from /etc/passwd
type PasswdEntry struct {
	Username string
	UID      int
	GID      int
	Gecos    string
	Home     string
	Shell    string
}

func addUser(username string, gecosString string, home string) (PasswdEntry, error) {
	args := []string{
		"--disabled-password", "--gecos", gecosString,
	}
	if home != "" {
		args = append(args, "--home", home)
	}
	args = append(args, username)

	out, err := exec.Command("adduser", args...).CombinedOutput()

	if err != nil {
		return PasswdEntry{}, fmt.Errorf("Error creating user %s(%w):\n%s", username, err, out)
	}

	out, err = exec.Command("getent", "passwd", username).CombinedOutput()
	if err != nil {
		return PasswdEntry{}, fmt.Errorf("Error getting system users:\n%w", err)
	}

	return parsePasswdEntry(out), nil
}

func removeUser(u string) error {
	out, err := exec.Command("deluser", "--remove-all-files", "--backup", u).CombinedOutput()

	if err != nil {
		return fmt.Errorf("Error removing user %s(%w):\n%s", u, err, out)
	}
	return nil
}

func getExistingUsers() (map[string]PasswdEntry, error) {
	out, err := exec.Command("getent", "passwd").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Error getting system users:\n%w", err)
	}
	lines := bytes.Split(out, []byte("\n"))
	existingUsers := make(map[string]PasswdEntry, len(lines))

	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		e := parsePasswdEntry(line)
		existingUsers[e.Username] = e
	}
	return existingUsers, nil
}

func parsePasswdEntry(line []byte) PasswdEntry {
	fields := bytes.Split(bytes.TrimSpace(line), []byte(":"))
	username := string(fields[0])
	uid, err := strconv.ParseInt(string(fields[2]), 10, 32)
	if err != nil {
		panic(fmt.Errorf("unexpected UID for user %s: %s (%w)", username, fields[2], err))
	}
	gid, err := strconv.ParseInt(string(fields[3]), 10, 32)
	if err != nil {
		panic(fmt.Errorf("unexpected GID for user %s: %s (%w)", username, fields[3], err))
	}

	shell := ""
	if len(fields) > 6 { // shell is optional according to PASSWD(5)
		shell = string(fields[6])
	}

	return PasswdEntry{
		Username: username,
		UID:      int(uid),
		GID:      int(gid),
		Gecos:    string(fields[4]),
		Home:     string(fields[5]),
		Shell:    shell,
	}
}

func ensureGroups(u PasswdEntry, wantedGroups []string) error {
	out, err := exec.Command("groups", u.Username).CombinedOutput()
	if err != nil {
		return fmt.Errorf("Erro finding groups (%w):\n%s", err, out)
	}
	outStr := string(bytes.TrimSpace(out))
	groupsStr := strings.Split(outStr, " : ")[1]
	currentGroups := strings.Split(groupsStr, " ")
	sort.Strings(currentGroups)

	sort.Strings(wantedGroups)

	if len(currentGroups) == len(wantedGroups) {
		for i := range currentGroups {
			if currentGroups[i] != wantedGroups[i] {
				// groups differ, apply wanted groups
				out, err := exec.Command("usermod", "--groups", strings.Join(wantedGroups, ","), u.Username).CombinedOutput()
				if err != nil {
					return fmt.Errorf("Error modifying groups (%w):\n%s", err, out)
				}
			}
		}
	}

	return nil
}

func ensureNoPassword(username string) error {
	enabledPassword, err := hasEnabledPassword(username)
	if err != nil {
		return err
	}
	if enabledPassword {
		log.Printf("INFO: User %s has enabled password. Locking.", username)
		out, err := exec.Command("usermod", "--lock", username).CombinedOutput()
		if err != nil {
			return fmt.Errorf("Error locking user %s:\n%s", username, out)
		}
	}
	return nil
}

func hasEnabledPassword(username string) (bool, error) {
	out, err := exec.Command("getent", "shadow", username).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("Error getting shadow info for %s:\n%s", username, out)
	}

	lines := strings.Split(string(out), "\n")
	fields := strings.Split(lines[0], ":")
	enp := fields[1] != "*" && !strings.HasPrefix(fields[1], "!")
	return enp, nil
}

func setAuthorizedKeys(u localUser, authorizedKeys *bytes.Buffer) error {
	sshDir := filepath.Join(u.Home, ".ssh")
	authorizedKeysFilename := filepath.Join(sshDir, "authorized_keys")

	if _, err := os.Stat(filepath.Dir(authorizedKeysFilename)); os.IsNotExist(err) {
		err := os.Mkdir(sshDir, 0700)
		if err != nil {
			return err
		}
		err = os.Chown(sshDir, u.UID, u.GID)
		if err != nil {
			return err
		}
	}

	return ensureFileContent(authorizedKeysFilename, authorizedKeys.Bytes())
}

func checkNoAuthorizedKeys(user PasswdEntry) error {
	authorizedKeysFilename := filepath.Join(user.Home, ".ssh", "authorized_keys")
	_, err := os.Stat(authorizedKeysFilename)
	if err == nil {
		return fmt.Errorf("user %s has authorized keys file at %s", user.Username, authorizedKeysFilename)
	} else if os.IsNotExist(err) {
		return nil
	} else {
		return err
	}
}

func checkNoPassword(username string) error {
	enabledPassword, err := hasEnabledPassword(username)
	if err != nil {
		return err
	}
	if enabledPassword {
		return fmt.Errorf("user %s has enabled password", username)
	}
	return nil
}
