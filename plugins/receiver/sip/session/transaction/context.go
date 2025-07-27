package transaction

type TransactionContext struct {
	state TransactionState
	tx    Transaction
}

func (ctx *TransactionContext) HandleEvent(event TransactionEvent) error {
	newState, err := ctx.state.HandleEvent(ctx, event)
	if err != nil {
		return err
	}
	ctx.transitionTo(newState)
	return nil
}

func (ctx *TransactionContext) transitionTo(newState TransactionState) {
	if ctx.state != nil {
		ctx.state.Exit(ctx)
	}
	ctx.state = newState
	newState.Enter(ctx)
}
