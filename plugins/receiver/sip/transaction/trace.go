package sip

import "github.com/apache/skywalking-satellite/plugins/receiver/sip/types"

type TraceListener struct {
}

func (l *TraceListener) OnDialogCreated(dialog types.Dialog) {}

func (l *TraceListener) OnDialogStateChanged(dialog types.Dialog) {}

func (l *TraceListener) OnDialogTerminated(dialog types.Dialog) {}

func (l *TraceListener) OnTransactionCreated(transaction types.Transaction) {}

func (l *TraceListener) OnTransactionStateChanged(transaction types.Transaction) {}

func (l *TraceListener) OnTransactionTerminated(transaction types.Transaction) {}

func (l *TraceListener) OnTransactionTimeout(transaction types.Transaction) {}

func (l *TraceListener) OnTransactionError(transaction types.Transaction, err error) {}
