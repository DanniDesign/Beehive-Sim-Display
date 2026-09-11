package forager

import (
	"context"

	s "github.com/DanniDesign/Beehive-Sim-Display/simulation"
	"github.com/gookit/slog"
	"github.com/qmuntal/stateless"
)

type ForagerFSM struct {
	fsm *stateless.StateMachine
	bee *s.Bee
}

func New(b *s.Bee) *stateless.StateMachine {
	f := ForagerFSM{
		bee: b,
		fsm: stateless.NewStateMachine(s.StateWandering),
	}
	f.init()
	return f.fsm
}

func (f *ForagerFSM) init() {
	f.fsm = stateless.NewStateMachine(s.StateWandering)

	f.fsm.Configure(s.StateWandering).
		Permit(s.EventHoneySupplyLow, s.StateMovingToExit).
		Permit(s.EventSickness, s.StateSick).
		Ignore(s.EventHoneySupplyOK).
		Ignore(s.EventSicknessOver).
		Ignore(s.EventHarvestedHoney).
		Ignore(s.EventWentOutside)
	f.fsm.Configure(s.StateMovingToExit).
		Permit(s.EventHoneySupplyOK, s.StateWandering).
		Ignore(s.EventWander).
		Permit(s.EventWentOutside, s.StateOutside).
		Permit(s.EventSickness, s.StateSick).
		Ignore(s.EventHoneySupplyLow).
		Ignore(s.EventSicknessOver).
		Ignore(s.EventHarvestedHoney)
	// NOTE: previously had Ignore(s.EventWander) registered twice here,
	// plus both Permit(s.EventSickness, ...) and Ignore(s.EventSickness)
	// for this same state. Either duplicate makes the library see two
	// candidate handlers for the same trigger with no guard to pick
	// between them, which panics at Fire-time with "Multiple permitted
	// exit transitions... Guard clauses must be mutually exclusive."
	// Each trigger is now registered exactly once per state.
	f.fsm.Configure(s.StateOutside).
		Permit(s.EventHarvestedHoney, s.StateBringingHoneyToStorage).
		Permit(s.EventSickness, s.StateSick).
		Ignore(s.EventWander).
		Ignore(s.EventHoneySupplyLow).
		Ignore(s.EventHoneySupplyOK).
		Ignore(s.EventWentOutside)
	f.fsm.Configure(s.StateBringingHoneyToStorage).
		Permit(s.EventStoredHoney, s.StateWandering).
		Permit(s.EventSickness, s.StateSick).
		Ignore(s.EventHoneySupplyLow).
		Ignore(s.EventHoneySupplyOK).
		Ignore(s.EventWander).
		Ignore(s.EventHarvestedHoney).
		Ignore(s.EventWentOutside)
	f.fsm.Configure(s.StateSick).
		Permit(s.EventSicknessOver, s.StateBringingHoneyToStorage, func(ctx context.Context, args ...any) bool {
			if len(args) == 0 {
				slog.Fatalf("FSM guard failed: no arguments provided")
				return false
			}
			bee, ok := args[0].(*s.Bee)
			if !ok {
				slog.Fatalf("Failed to parse bee argument in sickness state fsm.")
				return false
			}
			return bee.Carrying
		}).
		Permit(s.EventSicknessOver, s.StateWandering, func(ctx context.Context, args ...any) bool {
			if len(args) == 0 {
				slog.Fatalf("FSM guard failed: no arguments provided")
				return false
			}
			bee, ok := args[0].(*s.Bee)
			if !ok {
				slog.Fatalf("Failed to parse bee argument in sickness state fsm.")
				return false
			}
			return !bee.Carrying
		}).
		Ignore(s.EventHoneySupplyLow).
		Ignore(s.EventHoneySupplyOK).
		Ignore(s.EventWander).
		Ignore(s.EventHarvestedHoney).
		Ignore(s.EventWentOutside)
	f.fsm.OnTransitioned(func(ctx context.Context, transition stateless.Transition) {
		slog.Infof("FSM Transitioned: %v -> %v (Trigger: %v)", transition.Source, transition.Destination, transition.Trigger)
	})

	f.fsm.OnUnhandledTrigger(func(ctx context.Context, state stateless.State, trigger stateless.Trigger, unmetGuards []string) error {
		slog.Warnf("FSM Ignored/Unhandled Trigger! State: %v, Trigger: %v", state, trigger)
		return nil // Return nil so it doesn't crash, just logs
	})
}

func (f *ForagerFSM) State() s.State {
	return f.fsm.MustState().(s.State)
}

func (f *ForagerFSM) Fire(ctx context.Context, event s.Event, args ...any) error {
	return f.fsm.FireCtx(ctx, event, args...)
}
