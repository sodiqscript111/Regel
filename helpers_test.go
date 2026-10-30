package regel_test

import (
	"os"
	"testing"

	regel "github.com/sodiqscript111/Regel"
)

func withdrawFixed() *regel.Spec {
	return regel.New("WithdrawFixed").
		Var("balance", 100).
		Workers("a", "b").
		Step("withdraw", regel.Set("balance", "IF balance >= 60 THEN balance - 60 ELSE balance")).
		Always("NoOverdraft", "balance >= 0")
}

func tlcTools(t *testing.T) (java, jar string) {
	t.Helper()
	if os.Getenv("REGEL_SKIP_TLC") == "1" {
		t.Skip("REGEL_SKIP_TLC=1 is set")
	}
	java = os.Getenv("REGEL_JAVA")
	jar = os.Getenv("REGEL_TLA2TOOLS")
	if java == "" || jar == "" {
		t.Fatal("this test runs TLC: set REGEL_JAVA and REGEL_TLA2TOOLS, or set REGEL_SKIP_TLC=1 to skip it")
	}
	return java, jar
}
