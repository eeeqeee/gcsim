package def

type Logger interface {
	Debugw(msg string, args ...interface{})
	Infow(msg string, args ...interface{})
	Warnw(msg string, args ...interface{})
}

type LogSource int

const (
	LogProcs LogSource = iota
	LogDamageEvent
	LogHurtEvent
	LogHealEvent
	LogCalc
	LogReactionEvent
	LogElementEvent
	LogSnapshotEvent
	LogStatusEvent
	LogActionEvent
	LogQueueEvent
	LogEnergyEvent
	LogCharacterEvent
	LogEnemyEvent
	LogHookEvent
	LogSimEvent
	LogTaskEvent
	LogArtifactEvent
	LogWeaponEvent
	LogShieldEvent
	LogConstructEvent
	LogICDEvent
)

var LogSourceString = [...]string{
	"procs",
	"damage",
	"hurt",
	"heal",
	"calc",
	"reaction",
	"element",
	"snapshot",
	"status",
	"action",
	"queue",
	"energy",
	"character",
	"enemy",
	"hook",
	"sim",
	"task",
	"artifact",
	"weapon",
	"shield",
	"construct",
	"icd",
}

func (l LogSource) String() string {
	return LogSourceString[l]
}