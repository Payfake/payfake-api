package service

import (
	"errors"
	"testing"

	"github.com/payfake/payfake-api/internal/domain"
	"gorm.io/gorm"
)

type stubTransactionLookupRepo struct {
	findByAccessCode func(string) (*domain.Transaction, error)
	findByReference  func(string, string) (*domain.Transaction, error)
}

func (s stubTransactionLookupRepo) FindByAccessCode(accessCode string) (*domain.Transaction, error) {
	return s.findByAccessCode(accessCode)
}

func (s stubTransactionLookupRepo) FindByReference(reference, merchantID string) (*domain.Transaction, error) {
	return s.findByReference(reference, merchantID)
}

func (s stubTransactionLookupRepo) UpdateStatus(string, domain.TransactionStatus, any) error {
	return nil
}

func (s stubTransactionLookupRepo) FindByID(string, string) (*domain.Transaction, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s stubTransactionLookupRepo) Create(*domain.Transaction) error {
	return nil
}

func (s stubTransactionLookupRepo) UpdateChannel(string, domain.TransactionChannel) error {
	return nil
}

func TestFindPendingTransactionReturnsPendingAccessCodeMatch(t *testing.T) {
	svc := &ChargeService{
		transactionRepo: stubTransactionLookupRepo{
			findByAccessCode: func(string) (*domain.Transaction, error) {
				return &domain.Transaction{Reference: "TXN_123", Status: domain.TransactionPending}, nil
			},
			findByReference: func(string, string) (*domain.Transaction, error) {
				t.Fatal("did not expect reference lookup when access code matches")
				return nil, nil
			},
		},
	}

	tx, hadLookupInput, err := svc.findPendingTransaction("ACC_123", "", "MRC_123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !hadLookupInput {
		t.Fatal("expected lookup input to be true")
	}
	if tx == nil || tx.Reference != "TXN_123" {
		t.Fatalf("expected pending transaction match, got %#v", tx)
	}
}

func TestFindPendingTransactionFallsBackToReferenceOnMissingAccessCode(t *testing.T) {
	svc := &ChargeService{
		transactionRepo: stubTransactionLookupRepo{
			findByAccessCode: func(string) (*domain.Transaction, error) {
				return nil, gorm.ErrRecordNotFound
			},
			findByReference: func(string, string) (*domain.Transaction, error) {
				return &domain.Transaction{Reference: "TXN_456", Status: domain.TransactionPending}, nil
			},
		},
	}

	tx, hadLookupInput, err := svc.findPendingTransaction("ACC_123", "TXN_456", "MRC_123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !hadLookupInput {
		t.Fatal("expected lookup input to be true")
	}
	if tx == nil || tx.Reference != "TXN_456" {
		t.Fatalf("expected reference lookup to succeed, got %#v", tx)
	}
}

func TestFindPendingTransactionReturnsNotPending(t *testing.T) {
	svc := &ChargeService{
		transactionRepo: stubTransactionLookupRepo{
			findByAccessCode: func(string) (*domain.Transaction, error) {
				return &domain.Transaction{Reference: "TXN_123", Status: domain.TransactionSuccess}, nil
			},
			findByReference: func(string, string) (*domain.Transaction, error) {
				return nil, gorm.ErrRecordNotFound
			},
		},
	}

	_, _, err := svc.findPendingTransaction("ACC_123", "", "MRC_123")
	if !errors.Is(err, ErrTransactionNotPending) {
		t.Fatalf("expected ErrTransactionNotPending, got %v", err)
	}
}

func TestFindPendingTransactionReturnsLookupError(t *testing.T) {
	svc := &ChargeService{
		transactionRepo: stubTransactionLookupRepo{
			findByAccessCode: func(string) (*domain.Transaction, error) {
				return nil, errors.New("db down")
			},
			findByReference: func(string, string) (*domain.Transaction, error) {
				return nil, gorm.ErrRecordNotFound
			},
		},
	}

	_, _, err := svc.findPendingTransaction("ACC_123", "", "MRC_123")
	if err == nil || err.Error() == "db down" {
		t.Fatalf("expected wrapped lookup error, got %v", err)
	}
}

func TestFindPendingTransactionReturnsNoMatchWithoutError(t *testing.T) {
	svc := &ChargeService{
		transactionRepo: stubTransactionLookupRepo{
			findByAccessCode: func(string) (*domain.Transaction, error) {
				return nil, gorm.ErrRecordNotFound
			},
			findByReference: func(string, string) (*domain.Transaction, error) {
				return nil, gorm.ErrRecordNotFound
			},
		},
	}

	tx, hadLookupInput, err := svc.findPendingTransaction("ACC_123", "TXN_123", "MRC_123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !hadLookupInput {
		t.Fatal("expected lookup input to be true")
	}
	if tx != nil {
		t.Fatalf("expected no transaction match, got %#v", tx)
	}
}
