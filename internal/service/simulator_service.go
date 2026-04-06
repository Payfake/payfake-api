package service

import (
	"math/rand"
	"time"

	"github.com/GordenArcher/payfake/internal/domain"
	"github.com/GordenArcher/payfake/internal/repository"
)

// SimulationResult is what the simulator returns after resolving
// the outcome of a charge attempt. The service layer uses this
// to decide what status to write to the transaction and charge records
// and what error code to return in the response.
type SimulationResult struct {
	// Status is the terminal state the transaction should move to.
	Status domain.TransactionStatus
	// ErrorCode is populated only when Status is "failed".
	// It maps to one of our CHARGE_* response codes so the developer
	// knows exactly why the charge failed, insufficient funds,
	// momo timeout, invalid card etc.
	ErrorCode string
	// DelayMS is how long the simulator waited before resolving.
	// We apply this delay before returning so the caller experiences
	// the simulated latency naturally.
	DelayMS int
}

type SimulatorService struct {
	scenarioRepo *repository.ScenarioRepository
}

func NewSimulatorService(scenarioRepo *repository.ScenarioRepository) *SimulatorService {
	return &SimulatorService{scenarioRepo: scenarioRepo}
}

// ResolveOutcome is the brain of Payfake.
// It takes the merchant ID and payment channel and returns a
// SimulationResult that determines what happens to the transaction.
//
// Resolution priority (highest to lowest):
//  1. ForceStatus —? if set, always return this status. No randomness.
//     This is what the /control/transactions/:ref/force endpoint sets.
//  2. Failure rate —> random roll against the configured failure rate.
//     If the roll fails, pick a channel-appropriate error code.
//  3. Default —> succeed. If nothing overrides, the charge succeeds.
func (s *SimulatorService) ResolveOutcome(merchantID string, channel domain.TransactionChannel) SimulationResult {
	// Fetch the scenario config for this merchant.
	// If none exists or fetch fails we use safe defaults —
	// zero failure rate, zero delay, no forced status.
	// This means Payfake works out of the box without any configuration.
	scenario, err := s.scenarioRepo.FindByMerchantID(merchantID)
	if err != nil || scenario == nil {
		scenario = s.defaultScenario()
	}

	// Apply the configured delay first, before we resolve the outcome.
	// This simulates real-world network and processing latency.
	// Developers need to handle slow responses gracefully, timeouts,
	// loading states, webhook-based confirmation flows etc.
	if scenario.DelayMS > 0 {
		time.Sleep(time.Duration(scenario.DelayMS) * time.Millisecond)
	}

	// Priority 1: Force status overrides everything.
	// When set via the control panel, every charge for this merchant
	// returns exactly this status until it's cleared. Useful for
	// testing specific scenarios deterministically.
	if scenario.ForceStatus != "" {
		status := domain.TransactionStatus(scenario.ForceStatus)
		errorCode := scenario.ErrorCode

		// If forcing a failure but no specific error code is configured,
		// fall back to a generic channel-appropriate error code.
		if status == domain.TransactionFailed && errorCode == "" {
			errorCode = s.defaultErrorCode(channel)
		}

		return SimulationResult{
			Status:    status,
			ErrorCode: errorCode,
			DelayMS:   scenario.DelayMS,
		}
	}

	// Priority 2: Failure rate roll.
	// rand.Float64() returns a value in [0.0, 1.0).
	// If failure_rate is 0.3 (30%), then 30% of rolls will be < 0.3
	// and trigger a failure. The distribution is uniform so over
	// enough transactions the actual failure rate approaches the
	// configured rate closely.
	if scenario.FailureRate > 0 {
		roll := rand.Float64()
		if roll < scenario.FailureRate {
			return SimulationResult{
				Status:    domain.TransactionFailed,
				ErrorCode: s.defaultErrorCode(channel),
				DelayMS:   scenario.DelayMS,
			}
		}
	}

	// Priority 3: Default success.
	// If nothing above triggered a failure the charge succeeds.
	return SimulationResult{
		Status:    domain.TransactionSuccess,
		ErrorCode: "",
		DelayMS:   scenario.DelayMS,
	}
}

// defaultErrorCode returns the most realistic failure code for each channel.
// We pick the most common real-world failure for each channel so that
// even without scenario configuration developers encounter realistic errors.
func (s *SimulatorService) defaultErrorCode(channel domain.TransactionChannel) string {
	switch channel {
	case domain.ChannelCard:
		// "Do not honor" is the most common card decline in Ghana,
		// banks block online transactions by default on most cards.
		return string(domain.ChargeDoNotHonor)
	case domain.ChannelMobileMoney:
		// MoMo timeout is the most common MoMo failure, customers
		// miss the prompt, have no signal, or dismiss it by mistake.
		return string(domain.ChargeMomoTimeout)
	case domain.ChannelBankTransfer:
		return string(domain.ChargeBankTransferFailed)
	default:
		return string(domain.ChargeFailed)
	}
}

// defaultScenario returns a zero-config scenario, everything succeeds,
// no delay, no forced status. Used when a merchant has no scenario config.
func (s *SimulatorService) defaultScenario() *domain.ScenarioConfig {
	return &domain.ScenarioConfig{
		FailureRate: 0,
		DelayMS:     0,
		ForceStatus: "",
		ErrorCode:   "",
	}
}
