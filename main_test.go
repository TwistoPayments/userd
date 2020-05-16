package main

import "testing"

import "reflect"

import "sort"

func TestUsersMerged(t *testing.T) {
	groupMapping := map[string][]string{
		"devops@twisto.cz":     {"sudo", "dev"},
		"developers@twisto.cz": {"dev"},
	}

	fetchedUsers := map[string][]directoryUser{
		"devops@twisto.cz": {
			{FullName: "Pepa z Depa", GithubUsername: "https://github.com/pepazd", Email: "pepa@depo.cz"},
			{FullName: "Romulus", GithubUsername: "https://github.com/romulus", Email: "romulus@roma.it"},
		},
		"developers@twisto.cz": {
			{FullName: "Pepa z Depa", GithubUsername: "pepazd", Email: "pepa@depo.cz"},
			{FullName: "Remus", GithubUsername: "remus", Email: "remus@roma.it"},
		},
	}

	localUsers := mergeUsers(fetchedUsers, groupMapping)

	expectedResult := map[string]*localUser{
		"pepazd": {
			Username:        "pepazd",
			CanHavePassword: false,
			Email:           "pepa@depo.cz",
			FullName:        "Pepa z Depa",
			groups:          map[string]struct{}{"sudo": {}, "dev": {}},
		},
		"romulus": {
			Username:        "romulus",
			FullName:        "Romulus",
			Email:           "romulus@roma.it",
			CanHavePassword: false,
			groups:          map[string]struct{}{"sudo": {}, "dev": {}},
		},
		"remus": {
			Username:        "remus",
			FullName:        "Remus",
			Email:           "remus@roma.it",
			CanHavePassword: false,
			groups:          map[string]struct{}{"dev": {}},
		},
	}
	if !reflect.DeepEqual(localUsers, expectedResult) {
		t.Errorf("Unexpected merge result: %#v", localUsers)
	}
}

func TestGetGroups(t *testing.T) {
	u := localUser{
		Username:        "pepazd",
		CanHavePassword: false,
		groups:          map[string]struct{}{"sudo": {}, "dev": {}},
	}

	groups := u.getGroups()
	sort.Sort(sort.StringSlice(groups))

	if !reflect.DeepEqual(groups, []string{"dev", "sudo"}) {
		t.Errorf("Unexpected result: %#v", groups)
	}
}
