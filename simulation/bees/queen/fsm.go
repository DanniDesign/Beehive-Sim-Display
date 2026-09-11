package queen

import (
	"context"

	s "github.com/DanniDesign/Beehive-Sim-Display/simulation"
	"github.com/gookit/slog"
	"github.com/qmuntal/stateless"
)

type QueenFSM struct {
	fsm *stateless.StateMachine
	bee *s.Bee
}

func New(b *s.Bee) *stateless.StateMachine {
	q := QueenFSM{
		bee: b,
		fsm: stateless.NewStateMachine(s.StateWandering),
	}
	q.init()
	return q.fsm
}

func (q *QueenFSM) init() {

	q.fsm.Configure(s.StateWandering).
		Permit(s.EventSickness, s.StateSick).
		Permit(s.EventReceivedHoney, s.StateCarryingHoney).
		Ignore(s.EventSicknessOver)
	q.fsm.Configure(s.StateCarryingHoney).
		Permit(s.EventEggPlaced, s.StateWandering).
		Permit(s.EventSickness, s.StateSick).
		Ignore(s.EventSicknessOver)
	q.fsm.Configure(s.StateSick).
		Permit(s.EventSicknessOver, s.StateCarryingHoney, func(ctx context.Context, args ...any) bool {
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
		Ignore(s.EventSickness).
		Ignore(s.EventWander)

	// Was missing on this FSM (the other two have it): without this, an
	// event the Queen can't accept in her current state fails silently
	// instead of logging, which makes exactly this class of bug harder
	// to spot.
	q.fsm.OnUnhandledTrigger(func(ctx context.Context, state stateless.State, trigger stateless.Trigger, unmetGuards []string) error {
		slog.Warnf("FSM Ignored/Unhandled Trigger! State: %v, Trigger: %v", state, trigger)
		return nil
	})
}

func (q *QueenFSM) State() s.State {
	return q.fsm.MustState().(s.State)
}

func (q *QueenFSM) Fire(ctx context.Context, event s.Event, args ...any) error {
	return q.fsm.FireCtx(ctx, event, args...)
}
