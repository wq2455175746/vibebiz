package scheduledtask

import (
	"testing"
	"time"
)

func TestNormalizeScheduleDaily(t *testing.T) {
	got, err := NormalizeSchedule(ScheduleInput{
		Type: ScheduleDaily, Time: "09:05", Timezone: "Asia/Shanghai",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.CronExpr != "5 9 * * *" || got.Type != ScheduleDaily {
		t.Fatalf("schedule = %+v", got)
	}
}

func TestNormalizeScheduleWeekly(t *testing.T) {
	got, err := NormalizeSchedule(ScheduleInput{
		Type: ScheduleWeekly, Time: "18:30", Weekdays: []int{1, 3, 5}, Timezone: "Asia/Shanghai",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.CronExpr != "30 18 * * 1,3,5" {
		t.Fatalf("cron = %q", got.CronExpr)
	}
}

func TestNormalizeScheduleRejectsBadInput(t *testing.T) {
	cases := []ScheduleInput{
		{Type: ScheduleDaily, Time: "25:00", Timezone: "Asia/Shanghai"},
		{Type: ScheduleDaily, Time: "09:00", Timezone: "Bad/Zone"},
		{Type: ScheduleCron, CronExpression: "* * *", Timezone: "Asia/Shanghai"},
		{Type: ScheduleWeekly, Time: "09:00", Weekdays: nil, Timezone: "Asia/Shanghai"},
	}
	for _, input := range cases {
		if _, err := NormalizeSchedule(input); err == nil {
			t.Fatalf("expected error for %+v", input)
		}
	}
}

func TestNextRunUsesTimezone(t *testing.T) {
	schedule, err := NormalizeSchedule(ScheduleInput{
		Type: ScheduleDaily, Time: "09:00", Timezone: "Asia/Shanghai",
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	next, err := NextRun(schedule, now)
	if err != nil {
		t.Fatal(err)
	}
	if next.Format(time.RFC3339) != "2026-10-30T01:00:00Z" {
		t.Fatalf("next = %s", next.Format(time.RFC3339))
	}
}
