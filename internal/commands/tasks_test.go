package commands

import "testing"

func TestParseTaskStatusWithStatistics(t *testing.T) {
	status := ParseTaskStatus([]string{
		"Task list             rate/hz  max/us  avg/us maxload avgload  total/ms   late    run reqd/us",
		"00 - (         SYSTEM)    1000      10       4  0.2%  0.1%       100      0   1000       5",
		"01 - (            PID)    8000      20       8  2.5%  1.5%       200      1   2000      10",
		"Check Functions (RX, ...)          6       2          0.3%        50",
		"Total (excluding SERIAL)                       1.6%",
	})
	if !status.Statistics || !status.LateStatistics {
		t.Fatalf("statistics flags = %+v", status)
	}
	if len(status.Tasks) != 2 {
		t.Fatalf("tasks = %+v", status.Tasks)
	}
	pid := status.Tasks[1]
	if pid.ID != 1 || pid.Name != "PID" || pid.RateHz != 8000 {
		t.Fatalf("pid task = %+v", pid)
	}
	if pid.MaxLoadPercent == nil || *pid.MaxLoadPercent != 2.5 || pid.AverageLoadPercent == nil || *pid.AverageLoadPercent != 1.5 {
		t.Fatalf("pid load = %+v", pid)
	}
	if pid.LateCount == nil || *pid.LateCount != 1 || pid.RunCount == nil || *pid.RunCount != 2000 || pid.RequiredExecutionUS == nil || *pid.RequiredExecutionUS != 10 {
		t.Fatalf("pid late stats = %+v", pid)
	}
	if status.CheckFunctions == nil || status.CheckFunctions.AverageLoadPercent == nil || *status.CheckFunctions.AverageLoadPercent != 0.3 {
		t.Fatalf("check functions = %+v", status.CheckFunctions)
	}
	if status.Total == nil || status.Total.AverageLoadPercent != 1.6 {
		t.Fatalf("total = %+v", status.Total)
	}
}

func TestParseTaskStatusWithoutStatistics(t *testing.T) {
	status := ParseTaskStatus([]string{
		"Task list",
		"00 - (         SYSTEM)    1000",
	})
	if status.Statistics || status.LateStatistics {
		t.Fatalf("statistics flags = %+v", status)
	}
	if len(status.Tasks) != 1 || status.Tasks[0].Name != "SYSTEM" || status.Tasks[0].RateHz != 1000 {
		t.Fatalf("tasks = %+v", status.Tasks)
	}
	if status.Tasks[0].MaxExecutionUS != nil {
		t.Fatalf("unexpected max execution = %+v", status.Tasks[0])
	}
}
