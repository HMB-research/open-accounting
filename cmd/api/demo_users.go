package main

import (
	"fmt"
	"strconv"
)

type demoUserDefinition struct {
	number int
	email  string
	slug   string
	schema string
}

var demoUsers = []demoUserDefinition{
	{number: 1, email: "demo1@example.com", slug: "demo1", schema: "tenant_demo1"},
	{number: 2, email: "demo2@example.com", slug: "demo2", schema: "tenant_demo2"},
	{number: 3, email: "demo3@example.com", slug: "demo3", schema: "tenant_demo3"},
	{number: 4, email: "demo4@example.com", slug: "demo4", schema: "tenant_demo4"},
}

func demoUserNumbers() []int {
	numbers := make([]int, 0, len(demoUsers))
	for _, user := range demoUsers {
		numbers = append(numbers, user.number)
	}
	return numbers
}

func demoUserByNumber(userNum int) (demoUserDefinition, bool) {
	for _, user := range demoUsers {
		if user.number == userNum {
			return user, true
		}
	}
	return demoUserDefinition{}, false
}

func parseDemoUserNumber(raw string) (int, error) {
	userNum, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid user parameter")
	}
	if _, ok := demoUserByNumber(userNum); !ok {
		return 0, fmt.Errorf("invalid user parameter")
	}
	return userNum, nil
}

func demoUsersForSelection(raw string) ([]demoUserDefinition, []int, error) {
	if raw == "" {
		return demoUsers, demoUserNumbers(), nil
	}

	userNum, err := parseDemoUserNumber(raw)
	if err != nil {
		return nil, nil, err
	}

	user, _ := demoUserByNumber(userNum)
	return []demoUserDefinition{user}, []int{userNum}, nil
}
