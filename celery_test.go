package celery

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kit/log"

	"github.com/marselester/gopher-celery/protocol"
)

func TestExecuteTaskPanic(t *testing.T) {
	a := NewApp()
	a.Register(
		"myproject.apps.myapp.tasks.mytask",
		"important",
		func(ctx context.Context, p *TaskParam) {
			_ = p.Args()[100]
		},
	)

	m := protocol.Task{
		ID:   "0ad73c66-f4c9-4600-bd20-96746e720eed",
		Name: "myproject.apps.myapp.tasks.mytask",
		Args: []interface{}{"fizz"},
		Kwargs: map[string]interface{}{
			"b": "bazz",
		},
	}

	want := "unexpected task error"
	err := a.executeTask(context.Background(), &m)
	if !strings.HasPrefix(err.Error(), want) {
		t.Errorf("expected %q got %q", want, err)
	}
}

func TestProduceAndConsume(t *testing.T) {
	app := NewApp(WithLogger(log.NewJSONLogger(os.Stderr)))
	err := app.Delay(
		"myproject.apps.myapp.tasks.mytask",
		"important",
		2,
		3,
	)
	if err != nil {
		t.Fatal(err)
	}

	// The test finishes either when ctx times out or the task finishes.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	res := make(chan int, 1)

	app.Register(
		"myproject.apps.myapp.tasks.mytask",
		"important",
		func(ctx context.Context, p *TaskParam) {
			defer cancel()

			p.NameArgs("a", "b")
			sum := p.MustInt("a") + p.MustInt("b")
			res <- sum
		},
	)
	if err := app.Run(ctx); err != nil {
		t.Error(err)
	}

	var (
		want = 5
		got  = 0
	)
	select {
	case got = <-res:
	default:
	}
	if want != got {
		t.Errorf("expected sum %d got %d", want, got)
	}
}

func TestProduceAndConsume_100times(t *testing.T) {
	app := NewApp(WithLogger(log.NewJSONLogger(os.Stderr)))
	for i := 0; i < 100; i++ {
		err := app.Delay(
			"myproject.apps.myapp.tasks.mytask",
			"important",
			2,
			3,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	// The test finishes either when ctx times out or all the tasks finish.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	var sum int32
	app.Register(
		"myproject.apps.myapp.tasks.mytask",
		"important",
		func(ctx context.Context, p *TaskParam) {
			p.NameArgs("a", "b")
			atomic.AddInt32(
				&sum,
				int32(p.MustInt("a")+p.MustInt("b")),
			)
		},
	)
	if err := app.Run(ctx); err != nil {
		t.Error(err)
	}

	var want int32 = 500
	if want != sum {
		t.Errorf("expected sum %d got %d", want, sum)
	}
}
