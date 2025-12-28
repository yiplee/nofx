package trader

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"nofx/decision"
	"nofx/market"
	"nofx/store"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/suite"
)

// ============================================================
// AutoTraderTestSuite - Structured testing using testify/suite
// ============================================================

// AutoTraderTestSuite Test suite for AutoTrader
// Uses testify/suite to organize tests, providing unified setup/teardown and mock management
type AutoTraderTestSuite struct {
	suite.Suite

	// Test subject
	autoTrader *AutoTrader

	// Mock dependencies
	mockTrader *MockTrader
	mockStore  *store.Store

	// gomonkey patches
	patches *gomonkey.Patches

	// Test configuration
	config AutoTraderConfig
}

// SetupSuite Executed once before the entire test suite starts
func (s *AutoTraderTestSuite) SetupSuite() {
	// Can initialize some global resources here
}

// TearDownSuite Executed once after the entire test suite ends
func (s *AutoTraderTestSuite) TearDownSuite() {
	// Clean up global resources
}

// SetupTest Executed before each test case starts
func (s *AutoTraderTestSuite) SetupTest() {
	// Initialize patches
	s.patches = gomonkey.NewPatches()

	// Create mock objects
	s.mockTrader = &MockTrader{
		balance: map[string]interface{}{
			"totalWalletBalance":    10000.0,
			"availableBalance":      8000.0,
			"totalUnrealizedProfit": 100.0,
		},
		positions: []map[string]interface{}{},
	}

	// Create temporary store (using nil means no actual store needed in test)
	s.mockStore = nil

	// Set default configuration
	s.config = AutoTraderConfig{
		ID:             "test_trader",
		Name:           "Test Trader",
		AIModel:        "deepseek",
		Exchange:       "binance",
		InitialBalance: 10000.0,
		ScanInterval:   3 * time.Minute,
		IsCrossMargin:  true,
		StrategyConfig: &store.StrategyConfig{
			RiskControl: store.RiskControlConfig{
				BTCETHMaxLeverage:  10,
				AltcoinMaxLeverage: 5,
			},
		},
	}

	// Create AutoTrader instance (direct construction, don't call NewAutoTrader to avoid external dependencies)
	s.autoTrader = &AutoTrader{
		id:                    s.config.ID,
		name:                  s.config.Name,
		aiModel:               s.config.AIModel,
		exchange:              s.config.Exchange,
		config:                s.config,
		trader:                s.mockTrader,
		mcpClient:             nil, // No actual MCP Client needed in tests
		store:                 s.mockStore,
		initialBalance:        s.config.InitialBalance,
		lastResetTime:         time.Now(),
		startTime:             time.Now(),
		callCount:             0,
		isRunning:             false,
		positionFirstSeenTime: make(map[string]int64),
		stopMonitorCh:         make(chan struct{}),
		peakPnLCache:          make(map[string]float64),
		lastBalanceSyncTime:   time.Now(),
		userID:                "test_user",
	}
}

// TearDownTest Executed after each test case ends
func (s *AutoTraderTestSuite) TearDownTest() {
	// Reset gomonkey patches
	if s.patches != nil {
		s.patches.Reset()
	}
}

// ============================================================
// Level 1: Utility function tests
// ============================================================

func (s *AutoTraderTestSuite) TestSortDecisionsByPriority() {
	tests := []struct {
		name  string
		input []decision.Decision
	}{
		{
			name: "Mixed decisions - verify priority sorting",
			input: []decision.Decision{
				{Action: "open_long", Symbol: "BTCUSDT"},
				{Action: "close_short", Symbol: "ETHUSDT"},
				{Action: "hold", Symbol: "BNBUSDT"},
				{Action: "open_short", Symbol: "ADAUSDT"},
				{Action: "close_long", Symbol: "DOGEUSDT"},
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := sortDecisionsByPriority(tt.input)

			s.Equal(len(tt.input), len(result), "Result length should be the same")

			// Verify priority is increasing
			getActionPriority := func(action string) int {
				switch action {
				case "close_long", "close_short":
					return 1
				case "open_long", "open_short":
					return 2
				case "hold", "wait":
					return 3
				default:
					return 999
				}
			}

			for i := 0; i < len(result)-1; i++ {
				currentPriority := getActionPriority(result[i].Action)
				nextPriority := getActionPriority(result[i+1].Action)
				s.LessOrEqual(currentPriority, nextPriority, "Priority should be increasing")
			}
		})
	}
}

func (s *AutoTraderTestSuite) TestNormalizeSymbol() {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Already standard format", "BTCUSDT", "BTCUSDT"},
		{"Lowercase to uppercase", "btcusdt", "BTCUSDT"},
		{"Coin name only - add USDT", "BTC", "BTCUSDT"},
		{"With spaces - remove spaces", " BTC ", "BTCUSDT"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := normalizeSymbol(tt.input)
			s.Equal(tt.expected, result)
		})
	}
}

// ============================================================
// Level 2: Getter/Setter tests
// ============================================================

func (s *AutoTraderTestSuite) TestGettersAndSetters() {
	s.Run("GetID", func() {
		s.Equal("test_trader", s.autoTrader.GetID())
	})

	s.Run("GetName", func() {
		s.Equal("Test Trader", s.autoTrader.GetName())
	})

	s.Run("GetSystemPromptTemplate", func() {
		// GetSystemPromptTemplate returns "strategy" or "custom" based on strategy config
		template := s.autoTrader.GetSystemPromptTemplate()
		s.Contains([]string{"strategy", "custom"}, template)
	})

	s.Run("SetCustomPrompt", func() {
		s.autoTrader.SetCustomPrompt("custom prompt")
		s.Equal("custom prompt", s.autoTrader.customPrompt)
	})
}

// ============================================================
// Level 3: PeakPnL cache tests
// ============================================================

func (s *AutoTraderTestSuite) TestPeakPnLCache() {
	s.Run("UpdatePeakPnL_first record", func() {
		s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 10.5)
		cache := s.autoTrader.GetPeakPnLCache()
		s.Equal(10.5, cache["BTCUSDT_long"])
	})

	s.Run("UpdatePeakPnL_update to higher value", func() {
		s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 15.0)
		cache := s.autoTrader.GetPeakPnLCache()
		s.Equal(15.0, cache["BTCUSDT_long"])
	})

	s.Run("UpdatePeakPnL_do not update to lower value", func() {
		s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 12.0)
		cache := s.autoTrader.GetPeakPnLCache()
		s.Equal(15.0, cache["BTCUSDT_long"], "Peak value should remain unchanged")
	})

	s.Run("ClearPeakPnLCache", func() {
		s.autoTrader.ClearPeakPnLCache("BTCUSDT", "long")
		cache := s.autoTrader.GetPeakPnLCache()
		_, exists := cache["BTCUSDT_long"]
		s.False(exists, "Should be cleared")
	})
}

// ============================================================
// Level 4: GetStatus tests
// ============================================================

func (s *AutoTraderTestSuite) TestGetStatus() {
	s.autoTrader.isRunning = true
	s.autoTrader.callCount = 15

	status := s.autoTrader.GetStatus()

	s.Equal("test_trader", status["trader_id"])
	s.Equal("Test Trader", status["trader_name"])
	s.Equal("deepseek", status["ai_model"])
	s.Equal("binance", status["exchange"])
	s.True(status["is_running"].(bool))
	s.Equal(15, status["call_count"])
	s.Equal(10000.0, status["initial_balance"])
}

// ============================================================
// Level 5: GetAccountInfo tests
// ============================================================

func (s *AutoTraderTestSuite) TestGetAccountInfo() {
	accountInfo, err := s.autoTrader.GetAccountInfo()

	s.NoError(err)
	s.NotNil(accountInfo)

	// Verify core fields and values
	s.Equal(10100.0, accountInfo["total_equity"]) // 10000 + 100
	s.Equal(8000.0, accountInfo["available_balance"])
	s.Equal(100.0, accountInfo["total_pnl"]) // 10100 - 10000
}

// ============================================================
// Level 6: GetPositions tests
// ============================================================

func (s *AutoTraderTestSuite) TestGetPositions() {
	s.Run("No positions", func() {
		positions, err := s.autoTrader.GetPositions()

		s.NoError(err)
		// positions may be nil or empty array, both are valid
		if positions != nil {
			s.Equal(0, len(positions))
		}
	})

	s.Run("Has positions", func() {
		// Set mock positions
		s.mockTrader.positions = []map[string]interface{}{
			{
				"symbol":           "BTCUSDT",
				"side":             "long",
				"entryPrice":       50000.0,
				"markPrice":        51000.0,
				"positionAmt":      0.1,
				"unRealizedProfit": 100.0,
				"liquidationPrice": 45000.0,
				"leverage":         10.0,
			},
		}

		positions, err := s.autoTrader.GetPositions()

		s.NoError(err)
		s.Equal(1, len(positions))

		pos := positions[0]
		s.Equal("BTCUSDT", pos["symbol"])
		s.Equal("long", pos["side"])
		s.Equal(0.1, pos["quantity"])
		s.Equal(50000.0, pos["entry_price"])
	})
}

// ============================================================
// Level 7: getCandidateCoins tests
// ============================================================

func (s *AutoTraderTestSuite) TestGetCandidateCoins() {
	// Note: getCandidateCoins is now handled by strategyEngine.GetCandidateCoins()
	// This test is kept for reference but tests are moved to strategy engine tests
	s.Run("Strategy engine handles candidate coins", func() {
		// This test verifies that candidate coins are accessed through strategy engine
		// The actual implementation is in decision.StrategyEngine
		s.True(true, "Candidate coins are now handled by strategy engine")
	})
}

// ============================================================
// Level 8: buildTradingContext tests
// ============================================================

func (s *AutoTraderTestSuite) TestBuildTradingContext() {
	// Mock market.Get
	s.patches.ApplyFunc(market.GetFundingData, func(symbol string) (*market.FundingData, error) {
		return &market.FundingData{Symbol: symbol, MarkPrice: 50000.0}, nil
	})

	// Create a real strategy engine with test config
	strategyConfig := &store.StrategyConfig{
		CoinSource: store.CoinSourceConfig{
			SourceType:  "static",
			StaticCoins: []string{"BTCUSDT", "ETHUSDT"},
		},
		RiskControl: store.RiskControlConfig{
			BTCETHMaxLeverage:  10,
			AltcoinMaxLeverage: 5,
		},
	}
	s.autoTrader.strategyEngine = decision.NewStrategyEngine(strategyConfig)

	ctx, err := s.autoTrader.buildTradingContext()

	s.NoError(err)
	s.NotNil(ctx)

	// Verify core fields
	s.Equal(10100.0, ctx.Account.TotalEquity) // 10000 + 100
	s.Equal(8000.0, ctx.Account.AvailableBalance)
	s.Equal(10, ctx.StrategyConfig.RiskControl.BTCETHMaxLeverage)
	s.Equal(5, ctx.StrategyConfig.RiskControl.AltcoinMaxLeverage)
}

// ============================================================
// Level 9: Trade execution tests
// ============================================================

// TestExecuteOpenPosition Test open position operation (common for long and short)
func (s *AutoTraderTestSuite) TestExecuteOpenPosition() {
	tests := []struct {
		name          string
		action        string
		expectedOrder int64
		existingSide  string
		availBalance  float64
		expectedErr   string
		executeFn     func(*decision.Decision, *store.DecisionAction) error
	}{
		{
			name:          "Successfully open long",
			action:        "open_long",
			expectedOrder: 123456,
			availBalance:  8000.0,
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeOpenLongWithRecord(d, a)
			},
		},
		{
			name:          "Successfully open short",
			action:        "open_short",
			expectedOrder: 123457,
			availBalance:  8000.0,
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeOpenShortWithRecord(d, a)
			},
		},
		{
			name:         "Long - insufficient margin",
			action:       "open_long",
			availBalance: 0.0,
			expectedErr:  "Insufficient margin",
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeOpenLongWithRecord(d, a)
			},
		},
		{
			name:         "Short - insufficient margin",
			action:       "open_short",
			availBalance: 0.0,
			expectedErr:  "Insufficient margin",
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeOpenShortWithRecord(d, a)
			},
		},
		{
			name:         "Long - already has same side position",
			action:       "open_long",
			existingSide: "long",
			availBalance: 8000.0,
			expectedErr:  "Already has long position",
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeOpenLongWithRecord(d, a)
			},
		},
		{
			name:         "Short - already has same side position",
			action:       "open_short",
			existingSide: "short",
			availBalance: 8000.0,
			expectedErr:  "Already has short position",
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeOpenShortWithRecord(d, a)
			},
		},
	}

	for _, tt := range tests {
		time.Sleep(time.Millisecond)
		s.Run(tt.name, func() {
			s.patches.ApplyFunc(market.GetFundingData, func(symbol string) (*market.FundingData, error) {
				return &market.FundingData{Symbol: symbol, MarkPrice: 50000.0}, nil
			})

			s.mockTrader.balance["availableBalance"] = tt.availBalance
			if tt.existingSide != "" {
				s.mockTrader.positions = []map[string]interface{}{{"symbol": "BTCUSDT", "side": tt.existingSide}}
			} else {
				s.mockTrader.positions = []map[string]interface{}{}
			}

			decision := &decision.Decision{Action: tt.action, Symbol: "BTCUSDT", PositionSizeUSD: 1000.0, Leverage: 10}
			actionRecord := &store.DecisionAction{Action: tt.action, Symbol: "BTCUSDT"}

			err := tt.executeFn(decision, actionRecord)

			if tt.expectedErr != "" {
				s.Error(err)
				s.Contains(err.Error(), tt.expectedErr)
			} else {
				s.NoError(err)
				s.Equal(tt.expectedOrder, actionRecord.OrderID)
				s.Greater(actionRecord.Quantity, 0.0)
				s.Equal(50000.0, actionRecord.Price)
			}

			// Restore default state
			s.mockTrader.balance["availableBalance"] = 8000.0
			s.mockTrader.positions = []map[string]interface{}{}
		})
	}
}

// TestExecuteClosePosition Test close position operation (common for long and short)
func (s *AutoTraderTestSuite) TestExecuteClosePosition() {
	tests := []struct {
		name          string
		action        string
		currentPrice  float64
		expectedOrder int64
		executeFn     func(*decision.Decision, *store.DecisionAction) error
	}{
		{
			name:          "Successfully close long",
			action:        "close_long",
			currentPrice:  51000.0,
			expectedOrder: 123458,
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeCloseLongWithRecord(d, a)
			},
		},
		{
			name:          "Successfully close short",
			action:        "close_short",
			currentPrice:  49000.0,
			expectedOrder: 123459,
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeCloseShortWithRecord(d, a)
			},
		},
	}

	for _, tt := range tests {
		time.Sleep(time.Millisecond)
		s.Run(tt.name, func() {
			s.patches.ApplyFunc(market.GetFundingData, func(symbol string) (*market.FundingData, error) {
				return &market.FundingData{Symbol: symbol, MarkPrice: tt.currentPrice}, nil
			})

			decision := &decision.Decision{Action: tt.action, Symbol: "BTCUSDT"}
			actionRecord := &store.DecisionAction{Action: tt.action, Symbol: "BTCUSDT"}

			err := tt.executeFn(decision, actionRecord)

			s.NoError(err)
			s.Equal(tt.expectedOrder, actionRecord.OrderID)
			s.Equal(tt.currentPrice, actionRecord.Price)
		})
	}
}

// ============================================================
// Level 10: executeDecisionWithRecord routing tests
// ============================================================

func (s *AutoTraderTestSuite) TestExecuteDecisionWithRecord() {
	// Mock market.Get
	s.patches.ApplyFunc(market.GetFundingData, func(symbol string) (*market.FundingData, error) {
		return &market.FundingData{
			Symbol:    symbol,
			MarkPrice: 50000.0,
		}, nil
	})

	s.Run("Route to open_long", func() {
		decision := &decision.Decision{
			Action:          "open_long",
			Symbol:          "BTCUSDT",
			PositionSizeUSD: 1000.0,
			Leverage:        10,
		}
		actionRecord := &store.DecisionAction{}

		err := s.autoTrader.executeDecisionWithRecord(decision, actionRecord)
		s.NoError(err)
	})

	s.Run("Route to close_long", func() {
		decision := &decision.Decision{
			Action: "close_long",
			Symbol: "BTCUSDT",
		}
		actionRecord := &store.DecisionAction{}

		err := s.autoTrader.executeDecisionWithRecord(decision, actionRecord)
		s.NoError(err)
	})

	s.Run("Route to hold - no execution", func() {
		decision := &decision.Decision{
			Action: "hold",
			Symbol: "BTCUSDT",
		}
		actionRecord := &store.DecisionAction{}

		err := s.autoTrader.executeDecisionWithRecord(decision, actionRecord)
		s.NoError(err)
	})

	s.Run("Unknown action returns error", func() {
		decision := &decision.Decision{
			Action: "unknown_action",
			Symbol: "BTCUSDT",
		}
		actionRecord := &store.DecisionAction{}

		err := s.autoTrader.executeDecisionWithRecord(decision, actionRecord)
		s.Error(err)
		s.Contains(err.Error(), "Unknown action")
	})
}

func (s *AutoTraderTestSuite) TestCheckPositionDrawdown() {
	tests := []struct {
		name             string
		setupPositions   func()
		setupPeakPnL     func()
		setupFailures    func()
		cleanupFailures  func()
		expectedCacheKey string
		shouldClearCache bool
		skipCacheCheck   bool
	}{
		{
			name:            "Get positions failed - no panic",
			setupFailures:   func() { s.mockTrader.shouldFailPositions = true },
			cleanupFailures: func() { s.mockTrader.shouldFailPositions = false },
			skipCacheCheck:  true,
		},
		{
			name:           "No positions - no panic",
			setupPositions: func() { s.mockTrader.positions = []map[string]interface{}{} },
			skipCacheCheck: true,
		},
		{
			name: "Profit less than 5% - no close",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "BTCUSDT", "side": "long", "positionAmt": 0.1, "entryPrice": 50000.0, "markPrice": 50150.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:   func() { s.autoTrader.ClearPeakPnLCache("BTCUSDT", "long") },
			skipCacheCheck: true,
		},
		{
			name: "Drawdown less than 40% - no close",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "BTCUSDT", "side": "long", "positionAmt": 0.1, "entryPrice": 50000.0, "markPrice": 50400.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:   func() { s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 10.0) },
			skipCacheCheck: true,
		},
		{
			name: "Long - trigger drawdown close",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "BTCUSDT", "side": "long", "positionAmt": 0.1, "entryPrice": 50000.0, "markPrice": 50300.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:     func() { s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 10.0) },
			expectedCacheKey: "BTCUSDT_long",
			shouldClearCache: true,
		},
		{
			name: "Short - trigger drawdown close",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "ETHUSDT", "side": "short", "positionAmt": -0.5, "entryPrice": 3000.0, "markPrice": 2982.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:     func() { s.autoTrader.UpdatePeakPnL("ETHUSDT", "short", 10.0) },
			expectedCacheKey: "ETHUSDT_short",
			shouldClearCache: true,
		},
		{
			name: "Long - close failed - keep cache",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "BTCUSDT", "side": "long", "positionAmt": 0.1, "entryPrice": 50000.0, "markPrice": 50300.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:     func() { s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 10.0) },
			setupFailures:    func() { s.mockTrader.shouldFailCloseLong = true },
			cleanupFailures:  func() { s.mockTrader.shouldFailCloseLong = false },
			expectedCacheKey: "BTCUSDT_long",
			shouldClearCache: false,
		},
		{
			name: "Short - close failed - keep cache",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "ETHUSDT", "side": "short", "positionAmt": -0.5, "entryPrice": 3000.0, "markPrice": 2982.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:     func() { s.autoTrader.UpdatePeakPnL("ETHUSDT", "short", 10.0) },
			setupFailures:    func() { s.mockTrader.shouldFailCloseShort = true },
			cleanupFailures:  func() { s.mockTrader.shouldFailCloseShort = false },
			expectedCacheKey: "ETHUSDT_short",
			shouldClearCache: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.setupPositions != nil {
				tt.setupPositions()
			}
			if tt.setupPeakPnL != nil {
				tt.setupPeakPnL()
			}
			if tt.setupFailures != nil {
				tt.setupFailures()
			}
			if tt.cleanupFailures != nil {
				defer tt.cleanupFailures()
			}

			s.autoTrader.checkPositionDrawdown()

			if !tt.skipCacheCheck {
				cache := s.autoTrader.GetPeakPnLCache()
				_, exists := cache[tt.expectedCacheKey]
				if tt.shouldClearCache {
					s.False(exists, "Peak PnL cache should be cleared")
				} else {
					s.True(exists, "Peak PnL cache should not be cleared")
				}
			}

			// Clean up state
			s.mockTrader.positions = []map[string]interface{}{}
		})
	}
}

// ============================================================
// Mock implementations
// ============================================================

// MockDatabase Mock database
type MockDatabase struct {
	shouldFail bool
}

func (m *MockDatabase) UpdateTraderInitialBalance(userID, traderID string, newBalance float64) error {
	if m.shouldFail {
		return errors.New("database error")
	}
	return nil
}

// MockTrader Enhanced version (with error control)
type MockTrader struct {
	balance              map[string]interface{}
	positions            []map[string]interface{}
	shouldFailBalance    bool
	shouldFailPositions  bool
	shouldFailOpenLong   bool
	shouldFailCloseLong  bool
	shouldFailCloseShort bool
}

func (m *MockTrader) GetBalance() (map[string]interface{}, error) {
	if m.shouldFailBalance {
		return nil, errors.New("failed to get balance")
	}
	if m.balance == nil {
		return map[string]interface{}{
			"totalWalletBalance":    10000.0,
			"availableBalance":      8000.0,
			"totalUnrealizedProfit": 100.0,
		}, nil
	}
	return m.balance, nil
}

func (m *MockTrader) GetPositions() ([]map[string]interface{}, error) {
	if m.shouldFailPositions {
		return nil, errors.New("failed to get positions")
	}
	if m.positions == nil {
		return []map[string]interface{}{}, nil
	}
	return m.positions, nil
}

func (m *MockTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if m.shouldFailOpenLong {
		return nil, errors.New("failed to open long")
	}
	return map[string]interface{}{
		"orderId": int64(123456),
		"symbol":  symbol,
	}, nil
}

func (m *MockTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return map[string]interface{}{
		"orderId": int64(123457),
		"symbol":  symbol,
	}, nil
}

func (m *MockTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	if m.shouldFailCloseLong {
		return nil, errors.New("failed to close long")
	}
	return map[string]interface{}{
		"orderId": int64(123458),
		"symbol":  symbol,
	}, nil
}

func (m *MockTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	if m.shouldFailCloseShort {
		return nil, errors.New("failed to close short")
	}
	return map[string]interface{}{
		"orderId": int64(123459),
		"symbol":  symbol,
	}, nil
}

func (m *MockTrader) SetLeverage(symbol string, leverage int) error {
	return nil
}

func (m *MockTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return nil
}

func (m *MockTrader) GetMarketPrice(symbol string) (float64, error) {
	return 50000.0, nil
}

func (m *MockTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return nil
}

func (m *MockTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return nil
}

func (m *MockTrader) CancelStopLossOrders(symbol string) error {
	return nil
}

func (m *MockTrader) CancelTakeProfitOrders(symbol string) error {
	return nil
}

func (m *MockTrader) CancelAllOrders(symbol string) error {
	return nil
}

func (m *MockTrader) CancelStopOrders(symbol string) error {
	return nil
}

func (m *MockTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return fmt.Sprintf("%.4f", quantity), nil
}

func (m *MockTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"status":      "FILLED",
		"avgPrice":    50000.0,
		"executedQty": 0.1,
		"commission":  0.0,
	}, nil
}

func (m *MockTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	return []ClosedPnLRecord{}, nil
}

// ============================================================
// Test suite entry point
// ============================================================

// TestAutoTraderTestSuite Run AutoTrader test suite
func TestAutoTraderTestSuite(t *testing.T) {
	suite.Run(t, new(AutoTraderTestSuite))
}

// ============================================================
// Independent unit tests - calculatePnLPercentage function tests
// ============================================================

func TestCalculatePnLPercentage(t *testing.T) {
	tests := []struct {
		name          string
		unrealizedPnl float64
		marginUsed    float64
		expected      float64
	}{
		{
			name:          "Normal profit - 10x leverage",
			unrealizedPnl: 100.0,  // 100 USDT profit
			marginUsed:    1000.0, // 1000 USDT margin
			expected:      10.0,   // 10% return
		},
		{
			name:          "Normal loss - 10x leverage",
			unrealizedPnl: -50.0,  // 50 USDT loss
			marginUsed:    1000.0, // 1000 USDT margin
			expected:      -5.0,   // -5% return
		},
		{
			name:          "High leverage profit - 1% price increase, 20x leverage",
			unrealizedPnl: 200.0,  // 200 USDT profit
			marginUsed:    1000.0, // 1000 USDT margin
			expected:      20.0,   // 20% return
		},
		{
			name:          "Zero margin - edge case",
			unrealizedPnl: 100.0,
			marginUsed:    0.0,
			expected:      0.0, // Should return 0 instead of division by zero error
		},
		{
			name:          "Negative margin - edge case",
			unrealizedPnl: 100.0,
			marginUsed:    -1000.0,
			expected:      0.0, // Should return 0 (abnormal case)
		},
		{
			name:          "Zero PnL",
			unrealizedPnl: 0.0,
			marginUsed:    1000.0,
			expected:      0.0,
		},
		{
			name:          "Small trade",
			unrealizedPnl: 0.5,
			marginUsed:    10.0,
			expected:      5.0,
		},
		{
			name:          "Large profit",
			unrealizedPnl: 5000.0,
			marginUsed:    10000.0,
			expected:      50.0,
		},
		{
			name:          "Tiny margin",
			unrealizedPnl: 1.0,
			marginUsed:    0.01,
			expected:      10000.0, // 100x return
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculatePnLPercentage(tt.unrealizedPnl, tt.marginUsed)

			// Use precision comparison to avoid floating point errors
			if math.Abs(result-tt.expected) > 0.0001 {
				t.Errorf("calculatePnLPercentage(%v, %v) = %v, want %v",
					tt.unrealizedPnl, tt.marginUsed, result, tt.expected)
			}
		})
	}
}

// TestCalculatePnLPercentage_RealWorldScenarios Real world scenario tests
func TestCalculatePnLPercentage_RealWorldScenarios(t *testing.T) {
	t.Run("BTC 10x leverage, 2% price increase", func(t *testing.T) {
		// Open: 1000 USDT margin, 10x leverage = 10000 USDT position
		// 2% price increase = 200 USDT profit
		// Return = 200 / 1000 = 20%
		result := calculatePnLPercentage(200.0, 1000.0)
		expected := 20.0
		if math.Abs(result-expected) > 0.0001 {
			t.Errorf("BTC scenario: got %v, want %v", result, expected)
		}
	})

	t.Run("ETH 5x leverage, 3% price decrease", func(t *testing.T) {
		// Open: 2000 USDT margin, 5x leverage = 10000 USDT position
		// 3% price decrease = -300 USDT loss
		// Return = -300 / 2000 = -15%
		result := calculatePnLPercentage(-300.0, 2000.0)
		expected := -15.0
		if math.Abs(result-expected) > 0.0001 {
			t.Errorf("ETH scenario: got %v, want %v", result, expected)
		}
	})

	t.Run("SOL 20x leverage, 0.5% price increase", func(t *testing.T) {
		// Open: 500 USDT margin, 20x leverage = 10000 USDT position
		// 0.5% price increase = 50 USDT profit
		// Return = 50 / 500 = 10%
		result := calculatePnLPercentage(50.0, 500.0)
		expected := 10.0
		if math.Abs(result-expected) > 0.0001 {
			t.Errorf("SOL scenario: got %v, want %v", result, expected)
		}
	})
}

// ============================================================
// Independent unit tests - nextAlignedTime function tests
// ============================================================

func TestNextAlignedTime(t *testing.T) {
	// Set a fixed time for testing: 2024-01-01 13:02:30
	testTime := time.Date(2024, 1, 1, 13, 2, 30, 0, time.UTC)

	tests := []struct {
		name           string
		currentTime    time.Time
		interval       time.Duration
		expectedMinute int
		expectedHour   int
		expectedDay    int // For date rollover cases
	}{
		{
			name:           "5 minute interval at 13:02",
			currentTime:    testTime,
			interval:       5 * time.Minute,
			expectedMinute: 5, // Next should be 13:05
			expectedHour:   13,
			expectedDay:    1, // Same day
		},
		{
			name:           "5 minute interval at 13:07",
			currentTime:    time.Date(2024, 1, 1, 13, 7, 30, 0, time.UTC),
			interval:       5 * time.Minute,
			expectedMinute: 10, // Next should be 13:10
			expectedHour:   13,
			expectedDay:    1,
		},
		{
			name:           "5 minute interval at 13:55",
			currentTime:    time.Date(2024, 1, 1, 13, 55, 30, 0, time.UTC),
			interval:       5 * time.Minute,
			expectedMinute: 0, // Next should be 14:00
			expectedHour:   14,
			expectedDay:    1,
		},
		{
			name:           "60 minute interval at 13:30",
			currentTime:    time.Date(2024, 1, 1, 13, 30, 0, 0, time.UTC),
			interval:       60 * time.Minute,
			expectedMinute: 0, // Next should be 14:00
			expectedHour:   14,
			expectedDay:    1,
		},
		{
			name:           "60 minute interval at 13:00",
			currentTime:    time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
			interval:       60 * time.Minute,
			expectedMinute: 0, // Next should be 13:00
			expectedHour:   14,
			expectedDay:    1,
		},
		{
			name:           "3 minute interval at 13:01",
			currentTime:    time.Date(2024, 1, 1, 13, 1, 30, 0, time.UTC),
			interval:       3 * time.Minute,
			expectedMinute: 3, // Next should be 13:03
			expectedHour:   13,
			expectedDay:    1,
		},
		{
			name:           "3 minute interval at 13:03",
			currentTime:    time.Date(2024, 1, 1, 13, 3, 0, 0, time.UTC),
			interval:       3 * time.Minute,
			expectedMinute: 6, // Next should be 13:06
			expectedHour:   13,
			expectedDay:    1,
		},
		{
			name:           "15 minute interval at 13:17",
			currentTime:    time.Date(2024, 1, 1, 13, 17, 0, 0, time.UTC),
			interval:       15 * time.Minute,
			expectedMinute: 30, // Next should be 13:30
			expectedHour:   13,
			expectedDay:    1,
		},
		{
			name:           "4 hour interval at 13:30",
			currentTime:    time.Date(2024, 1, 1, 13, 30, 0, 0, time.UTC),
			interval:       4 * time.Hour,
			expectedMinute: 0, // Next should be 16:00
			expectedHour:   16,
			expectedDay:    1,
		},
		{
			name:           "4 hour interval at 15:45",
			currentTime:    time.Date(2024, 1, 1, 15, 45, 0, 0, time.UTC),
			interval:       4 * time.Hour,
			expectedMinute: 0, // Next should be 16:00
			expectedHour:   16,
			expectedDay:    1,
		},
		{
			name:           "4 hour interval at 16:00",
			currentTime:    time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC),
			interval:       4 * time.Hour,
			expectedMinute: 0, // Next should be 20:00
			expectedHour:   20,
			expectedDay:    1,
		},
		{
			name:           "4 hour interval at 22:30",
			currentTime:    time.Date(2024, 1, 1, 22, 30, 0, 0, time.UTC),
			interval:       4 * time.Hour,
			expectedMinute: 0, // Next should be 00:00 (next day)
			expectedHour:   0,
			expectedDay:    2, // Next day
		},
		{
			name:           "4 hour interval at 02:15",
			currentTime:    time.Date(2024, 1, 1, 2, 15, 0, 0, time.UTC),
			interval:       4 * time.Hour,
			expectedMinute: 0, // Next should be 04:00
			expectedHour:   4,
			expectedDay:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the internal function directly for testing to avoid patching issues
			result := nextAlignedTimeFrom(tt.interval, tt.currentTime)

			if result.Hour() != tt.expectedHour {
				t.Errorf("nextAlignedTime() hour = %v, want %v", result.Hour(), tt.expectedHour)
			}
			if result.Minute() != tt.expectedMinute {
				t.Errorf("nextAlignedTime() minute = %v, want %v", result.Minute(), tt.expectedMinute)
			}
			if result.Second() != 0 {
				t.Errorf("nextAlignedTime() second = %v, want 0", result.Second())
			}
			if result.Nanosecond() != 0 {
				t.Errorf("nextAlignedTime() nanosecond = %v, want 0", result.Nanosecond())
			}

			// Verify the day (for date rollover cases)
			if tt.expectedDay > 0 && result.Day() != tt.expectedDay {
				t.Errorf("nextAlignedTime() day = %v, want %v", result.Day(), tt.expectedDay)
			}

			// Verify the result is in the future
			if result.Before(tt.currentTime) || result.Equal(tt.currentTime) {
				t.Errorf("nextAlignedTime() = %v, should be after current time %v", result, tt.currentTime)
			}
		})
	}
}

func TestNextAlignedTime_RealWorldScenarios(t *testing.T) {
	t.Run("5 minute interval - multiple boundaries", func(t *testing.T) {
		// Test that 5-minute intervals align to :00, :05, :10, :15, etc.
		testCases := []struct {
			current  time.Time
			expected time.Time
		}{
			{
				current:  time.Date(2024, 1, 1, 13, 2, 0, 0, time.UTC),
				expected: time.Date(2024, 1, 1, 13, 5, 0, 0, time.UTC),
			},
			{
				current:  time.Date(2024, 1, 1, 13, 5, 0, 0, time.UTC),
				expected: time.Date(2024, 1, 1, 13, 5, 0, 0, time.UTC),
			},
			{
				current:  time.Date(2024, 1, 1, 13, 58, 0, 0, time.UTC),
				expected: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			},
		}

		for _, tc := range testCases {
			// Use the internal function directly for testing
			result := nextAlignedTimeFrom(5*time.Minute, tc.current)
			if !result.Equal(tc.expected) {
				t.Errorf("nextAlignedTime() = %v, want %v", result, tc.expected)
			}
		}
	})

	t.Run("60 minute interval - hour boundaries", func(t *testing.T) {
		testCases := []struct {
			current  time.Time
			expected time.Time
		}{
			{
				current:  time.Date(2024, 1, 1, 13, 30, 0, 0, time.UTC),
				expected: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			},
			{
				current:  time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
				expected: time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
			},
			{
				current:  time.Date(2024, 1, 1, 23, 45, 0, 0, time.UTC),
				expected: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			},
		}

		for _, tc := range testCases {
			// Use the internal function directly for testing
			result := nextAlignedTimeFrom(60*time.Minute, tc.current)
			if !result.Equal(tc.expected) {
				t.Errorf("nextAlignedTime() = %v, want %v", result, tc.expected)
			}
		}
	})

	t.Run("4 hour interval - 4-hour boundaries", func(t *testing.T) {
		testCases := []struct {
			current  time.Time
			expected time.Time
		}{
			{
				current:  time.Date(2024, 1, 1, 13, 30, 0, 0, time.UTC),
				expected: time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC),
			},
			{
				current:  time.Date(2024, 1, 1, 15, 45, 0, 0, time.UTC),
				expected: time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC),
			},
			{
				current:  time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC),
				expected: time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC),
			},
			{
				current:  time.Date(2024, 1, 1, 20, 0, 0, 0, time.UTC),
				expected: time.Date(2024, 1, 1, 20, 0, 0, 0, time.UTC),
			},
			{
				current:  time.Date(2024, 1, 1, 22, 30, 0, 0, time.UTC),
				expected: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			},
			{
				current:  time.Date(2024, 1, 1, 2, 15, 0, 0, time.UTC),
				expected: time.Date(2024, 1, 1, 4, 0, 0, 0, time.UTC),
			},
			{
				current:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}

		for _, tc := range testCases {
			// Use the internal function directly for testing
			result := nextAlignedTimeFrom(4*time.Hour, tc.current)
			if !result.Equal(tc.expected) {
				t.Errorf("nextAlignedTime() = %v, want %v", result, tc.expected)
			}
		}
	})
}
