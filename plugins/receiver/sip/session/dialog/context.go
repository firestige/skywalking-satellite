package dialog

type DialogContext struct {
	state  DialogState
	dialog Dialog
}

func (ctx *DialogContext) HandleEvent(event DialogEvent) error {
	newState, err := ctx.state.HandleEvent(ctx, event)
	if err != nil {
		return err
	}
	ctx.transitionTo(newState)
	return nil
}

func (ctx *DialogContext) transitionTo(newState DialogState) {
	if ctx.state != nil {
		ctx.state.Exit(ctx)
	}
	ctx.state = newState
	newState.Enter(ctx)
}
