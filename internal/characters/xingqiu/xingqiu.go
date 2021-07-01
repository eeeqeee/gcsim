package xingqiu

import (
	"fmt"

	"github.com/genshinsim/gsim/pkg/character"
	"github.com/genshinsim/gsim/pkg/combat"
	"github.com/genshinsim/gsim/pkg/def"

	"go.uber.org/zap"
)

type char struct {
	*character.Tmpl
	numSwords          int
	nextRegen          bool
	burstCounter       int
	burstICDResetTimer int //if c.S.F > this, then reset counter to = 0
	orbitalActive      bool
	burstSwordICD      int
}

func init() {
	combat.RegisterCharFunc("xingqiu", NewChar)
}

func NewChar(s def.Sim, log *zap.SugaredLogger, p def.CharacterProfile) (def.Character, error) {
	c := char{}
	t, err := character.NewTemplateChar(s, log, p)
	if err != nil {
		return nil, err
	}
	c.Tmpl = t
	c.Energy = 80
	c.MaxEnergy = 80
	c.Weapon.Class = def.WeaponClassSword
	c.BurstCon = 3
	c.SkillCon = 5
	c.NormalHitNum = 5

	a4 := make([]float64, def.EndStatType)
	a4[def.HydroP] = 0.2
	c.AddMod(def.CharStatMod{
		Key: "a4",
		Amount: func(a def.AttackTag) ([]float64, bool) {
			return a4, true
		},
		Expiry: -1,
	})
	c.burstHook()

	/** c6
	Activating 2 of Guhua Sword: Raincutter's sword rain attacks greatly increases the DMG of the third.
	Xingqiu regenerates 3 Energy when sword rain attacks hit opponents.
	**/

	return &c, nil
}

var delay = [][]int{{8}, {24}, {24, 43}, {36}, {43, 78}}

func (c *char) ActionFrames(a def.ActionType, p map[string]int) int {
	switch a {
	case def.ActionAttack:
		f := 0
		switch c.NormalCounter {
		//TODO: need to add atkspd mod
		case 0:
			f = 9
		case 1:
			f = 25
		case 2:
			f = 44
		case 3:
			f = 37
		case 4:
			f = 79
		}
		f = int(float64(f) / (1 + c.Stats[def.AtkSpd]))
		return f
	case def.ActionCharge:
		return 63
	case def.ActionSkill:
		return 77 //should be 82
	case def.ActionBurst:
		return 39 //ok
	default:
		c.Log.Warnw("unknown action", "event", def.LogActionEvent, "frame", c.Sim.Frame(), "action", a)
		return 0
	}
}

func (c *char) Attack(p map[string]int) int {
	//apply attack speed
	f := c.ActionFrames(def.ActionAttack, p)

	d := c.Snapshot(
		fmt.Sprintf("Normal %v", c.NormalCounter),
		def.AttackTagNormal,
		def.ICDTagNormalAttack,
		def.ICDGroupDefault,
		def.StrikeTypeSlash,
		def.Physical,
		25,
		0,
	)

	for i, mult := range attack[c.NormalCounter] {
		x := d.Clone()
		x.Mult = mult[c.TalentLvlAttack()]
		c.QueueDmg(&x, delay[c.NormalCounter][i])
	}

	//add a 75 frame attackcounter reset
	c.AdvanceNormalIndex()
	//return animation cd
	//this also depends on which hit in the chain this is
	return f
}

func (c *char) orbitalfunc(src int) func() {
	return func() {
		c.Log.Debugw("orbital checking tick", "frame", c.Sim.Frame(), "event", def.LogCharacterEvent, "expiry", c.Sim.Status("xqorb"), "src", src)
		if c.Sim.Status("xqorb") == 0 {
			c.orbitalActive = false
			return
		}
		//queue up one damage instance
		d := c.Snapshot(
			"Xingqiu Skill (Orbital)",
			def.AttackTagNone,
			def.ICDTagNormalAttack,
			def.ICDGroupDefault,
			def.StrikeTypeDefault,
			def.Hydro,
			25,
			0,
		)

		c.QueueDmg(&d, 1)
		c.Log.Debugw("orbital ticked", "frame", c.Sim.Frame(), "event", def.LogCharacterEvent, "next expected tick", c.Sim.Frame()+150, "expiry", c.Sim.Status("xqorb"), "src", src)
		//queue up next instance
		c.AddTask(c.orbitalfunc(src), "xq-skill-orbital", 150)
	}
}

func (c *char) applyOrbital() {
	f := c.Sim.Frame()
	c.Log.Debugw("Applying orbital", "frame", f, "event", def.LogCharacterEvent, "current status", c.Sim.Status("xqorb"))
	//check if blood blossom already active, if active extend duration by 8 second
	//other wise start first tick func
	if !c.orbitalActive {
		//TODO: does BB tick immediately on first application?
		c.AddTask(c.orbitalfunc(f), "xq-skill-orbital", 40)
		c.orbitalActive = true
		c.Log.Debugw("orbital applied", "frame", f, "event", def.LogCharacterEvent, "expected end", f+900, "next expected tick", f+40)
	}
	c.Sim.AddStatus("xqorb", 900)
	c.Log.Debugw("orbital duration extended", "frame", f, "event", def.LogCharacterEvent, "new expiry", c.Sim.Status("xqorb"))
}

func (c *char) Skill(p map[string]int) int {
	//applies wet to self 30 frame after cast, sword applies wet every 2.5seconds, so should be 7 seconds
	orbital := p["orbital"]
	if orbital == 1 {
		c.applyOrbital()
	}

	f := c.ActionFrames(def.ActionSkill, p)

	d := c.Snapshot(
		"Guhua Sword: Fatal Rainscreen",
		def.AttackTagElementalArt,
		def.ICDTagNone,
		def.ICDGroupDefault,
		def.StrikeTypeSlash,
		def.Hydro,
		25,
		rainscreen[0][c.TalentLvlSkill()],
	)
	if c.Base.Cons >= 4 {
		//check if ult is up, if so increase multiplier
		if c.Sim.Status("xqburst") > 0 {
			d.Mult = d.Mult * 1.5
		}
	}
	d2 := d.Clone()
	d2.Mult = rainscreen[1][c.TalentLvlSkill()]

	c.QueueDmg(&d, 19)
	c.QueueDmg(&d2, 39)

	c.QueueParticle(c.Base.Name, 5, def.Hydro, 100)

	//should last 15s, cd 21s
	c.SetCD(def.ActionSkill, 21*60)
	return f
}

func (c *char) burstHook() {
	c.Sim.AddEventHook(func(s def.Sim) bool {
		//check if buff is up
		if c.Sim.Status("xqburst") <= 0 {
			return false
		}
		//check if off ICD
		if c.burstSwordICD > s.Frame() {
			return false
		}

		const delay = 5 //wait 5 frames into attack animation

		//trigger swords, only first sword applies hydro
		for i := 0; i < c.numSwords; i++ {

			wave := i

			d := c.Snapshot(
				"Guhua Sword: Raincutter",
				def.AttackTagElementalBurst,
				def.ICDTagElementalBurst,
				def.ICDGroupDefault,
				def.StrikeTypePierce,
				def.Hydro,
				25,
				burst[c.TalentLvlBurst()],
			)
			d.Targets = 0 //only hit main target
			d.OnHitCallback = func(t def.Target) {
				//check energy
				if c.nextRegen && wave == 0 {
					c.AddEnergy(3)
				}
				//check c2
				if c.Base.Cons >= 2 {
					t.AddResMod("xingqiu-c2", def.ResistMod{
						Ele:      def.Hydro,
						Value:    -0.15,
						Duration: 4 * 60,
					})
				}
			}

			c.QueueDmg(&d, delay+20+i)

			c.burstCounter++
		}

		//figure out next wave # of swords
		switch c.numSwords {
		case 2:
			c.numSwords = 3
			c.nextRegen = false
		case 3:
			if c.Base.Cons == 6 {
				c.numSwords = 5
				c.nextRegen = true
			} else {
				c.numSwords = 2
				c.nextRegen = false
			}
		case 5:
			c.numSwords = 2
			c.nextRegen = false
		}

		//estimated 1 second ICD
		c.burstSwordICD = c.Sim.Frame() + 60

		return false
	}, "Xingqiu-Burst", def.PostAttackHook)
}

func (c *char) Burst(p map[string]int) int {
	f := c.ActionFrames(def.ActionBurst, p)
	//apply hydro every 3rd hit
	//triggered on normal attack
	//also applies hydro on cast if p=1
	orbital := p["orbital"]

	if orbital == 1 {
		c.applyOrbital()
	}
	//how we doing that?? trigger 0 dmg?

	/**
	The number of Hydro Swords summoned per wave follows a specific pattern, usually alternating between 2 and 3 swords.
	At C6, this is upgraded and follows a pattern of 2 → 3 → 5… which then repeats.

	There is an approximately 1 second interval between summoned Hydro Sword waves, so that means a theoretical maximum of 15 or 18 waves.

	Each wave of Hydro Swords is capable of applying one (1) source of Hydro status, and each individual sword is capable of getting a crit.
	**/

	/** c2
	Extends the duration of Guhua Sword: Raincutter by 3s.
	Decreases the Hydro RES of opponents hit by sword rain attacks by 15% for 4s.
	**/
	dur := 15
	if c.Base.Cons >= 2 {
		dur += 3
	}
	dur = dur * 60
	c.Sim.AddStatus("xqburst", dur)
	c.Log.Debugw("Xingqiu burst activated", "frame", c.Sim.Frame(), "event", def.LogCharacterEvent, "expiry", c.Sim.Frame()+dur)

	c.burstCounter = 0
	c.numSwords = 2

	// c.CD[combat.BurstCD] = c.S.F + 20*60
	c.SetCD(def.ActionBurst, 20*60)
	c.Energy = 0
	return f
}