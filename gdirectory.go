package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

const maxResults = 500

type externalServices struct {
	GithubUsername string `json:"github_username"`
}

type directoryUser struct {
	FullName       string
	GithubUsername string
	Email          string
}

func getUsersForGroup(srv *admin.Service, group string) ([]directoryUser, error) {
	r, err := srv.Members.List(group).Do()

	if err != nil {
		return nil, fmt.Errorf("unable to retrieve group %s: %w", group, err)
	}

	if len(r.Members) > (maxResults - 15) {
		log.Printf("WARN: Retrieved %d/%d members (%s). Time to implement pagination. ;-)", len(r.Members), maxResults, group)
	}

	if len(r.Members) == maxResults {
		return nil, fmt.Errorf("retrieved as many members as maxResults (%s) - time to implement pagination", group)
	}

	users := make([]directoryUser, 0, len(r.Members))

	for _, m := range r.Members {
		r, err := srv.Users.Get(m.Email).Projection("full").Do()
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve details for %s: %w", m.Email, err)
		}

		servicesEncoded := r.CustomSchemas["external_services"]
		currentUser := directoryUser{
			FullName: r.Name.FullName,
			Email:    m.Email,
		}

		if servicesEncoded != nil {
			services := externalServices{}
			err = json.Unmarshal(servicesEncoded, &services)
			if err != nil {
				log.Fatalf("Unable to unmarshal services for %s: %v", m.Email, err)
			}
			currentUser.GithubUsername = services.GithubUsername
		}
		users = append(users, currentUser)
	}
	return users, nil
}

func mergeUsers(users map[string][]directoryUser, groupMapping map[string][]string) map[string]localUser {
	usermap := make(map[string]localUser)

	for directoryGroup, ulist := range users {
		for _, u := range ulist {
			if strings.TrimSpace(u.GithubUsername) == "" {
				log.Printf("WARNING: Skipping %s because this user doesn't have github_username field set up.", u.Email)
				continue
			}

			// TODO: Another field?
			localName := strings.Replace(strings.ToLower(u.GithubUsername), "https://github.com/", "", 1)

			local, ok := usermap[localName]
			if !ok {
				local = localUser{
					Username:        localName,
					FullName:        u.FullName,
					Email:           u.Email,
					CanHavePassword: false,
					SSHKeys:         nil,
				}
				usermap[localName] = local
			}

			localGroups, ok := groupMapping[directoryGroup]

			if !ok {
				log.Printf("ERROR: %#v\n%+v\n", groupMapping, localGroups)
				log.Panicf("Unknown mapping for group %s", directoryGroup)
			}

			for _, localGroup := range localGroups {
				local.addGroup(localGroup)
			}
		}
	}
	return usermap
}

func getUsers(srv *admin.Service, groups map[string][]string) (map[string]localUser, error) {
	usermap := make(map[string][]directoryUser)

	for group := range groups {
		users, err := getUsersForGroup(srv, group)

		if err != nil {
			return nil, fmt.Errorf("can't retrieve users of %s: %w", group, err)
		}
		log.Printf("DEBUG: Users of %s: %v\n", group, users)
		usermap[group] = users
	}
	return mergeUsers(usermap, groups), nil
}

func getDirectoryUsers(guconfig gooleUsersConfig) (map[string]localUser, error) {
	ctx := context.Background()
	credfile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	b, err := ioutil.ReadFile(credfile)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %w", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.JWTConfigFromJSON(
		b,
		admin.AdminDirectoryUserReadonlyScope,
		admin.AdminDirectoryGroupMemberReadonlyScope,
	)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	config.Subject = os.Getenv("GOOGLE_ADMIN_SUBJECT")

	ts := config.TokenSource(ctx)

	srv, err := admin.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("got NewService: %w", err)
	}

	return getUsers(srv, guconfig.Groups)
}
