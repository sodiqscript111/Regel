package main

import (
	"testing"

	"github.com/sodiqscript111/Regel/regeltest"
)

func TestBrokenWithdrawOverdraws(t *testing.T) {
	regeltest.Fails(t, broken(), "NoOverdraft")
}

func TestFixedWithdrawHolds(t *testing.T) {
	regeltest.Holds(t, fixed())
}
