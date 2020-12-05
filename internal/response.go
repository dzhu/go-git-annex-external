package internal

import "strings"

func Response0(f func()) CommandSpec {
	return CommandSpec{0, func(args []string) { f() }}
}

func Response1(f func(s string)) CommandSpec {
	return CommandSpec{1, func(args []string) { f(args[0]) }}
}

func Response2(f func(s1, s2 string)) CommandSpec {
	return CommandSpec{
		2, func(args []string) { f(args[0], args[1]) },
	}
}

func Response3(f func(s1, s2, s3 string)) CommandSpec {
	return CommandSpec{
		3, func(args []string) { f(args[0], args[1], args[2]) },
	}
}

func ResponseSplit(f func(s []string)) CommandSpec {
	return CommandSpec{
		1, func(args []string) { f(strings.Split(args[0], " ")) },
	}
}
