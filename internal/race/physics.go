package race

import "math"

// CarSpec holds the per-car tunables that scale the kinematic model.
// `BaseStats` from the cars table (top_speed, handling, acceleration) is
// decoded once at race start into this struct.
type CarSpec struct {
	MaxAccel      float64 // m/s²
	MaxSteerRate  float64 // rad/s
	MaxSpeed      float64 // m/s
}

// DefaultSpec returns the MVP's hard-coded car spec. Production would
// decode the JSON column.
func DefaultSpec() CarSpec {
	return CarSpec{MaxAccel: 12.0, MaxSteerRate: 1.5, MaxSpeed: 30.0}
}

// Step integrates `state` forward by `dt` seconds using `input` and
// `spec`. Bounds the speed to ±MaxSpeed and wraps heading to
// [-π, π]. Pure function — deterministic, no I/O, no time.
func Step(state *PlayerState, input PlayerInput, spec CarSpec, dt float64) {
	// Throttle (-1..+1) + brake are projected onto a single
	// longitudinal axis (positive = forward).
	drive := float64(input.Throttle - input.Brake)
	accel := drive * spec.MaxAccel

	// Convert mph-style max speed to per-second velocity.
	state.Velocity = clampF(state.Velocity+accel*dt, -spec.MaxSpeed, spec.MaxSpeed)

	// Steering is symmetric in [-1, 1]. Only meaningful at non-zero
	// velocity so the car doesn't pirouette while idle.
	headingDelta := float64(input.Steering) * spec.MaxSteerRate * dt
	state.Heading = wrapAngle(state.Heading + headingDelta)

	// Integrate position. Heading is yaw around the world Y axis.
	speed := state.Velocity
	state.Position.X += float32(speed * dt * math.Cos(state.Heading))
	state.Position.Z += float32(speed * dt * math.Sin(state.Heading))
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func wrapAngle(a float64) float64 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}
