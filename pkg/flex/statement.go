package flex

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// Num is a Flex numeric attribute; empty or "--" values parse as 0.
type Num float64

func (n *Num) UnmarshalXMLAttr(a xml.Attr) error {
	s := strings.ReplaceAll(strings.TrimSpace(a.Value), ",", "")
	if s == "" || s == "--" {
		*n = 0
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("flex attr %s=%q: %w", a.Name.Local, a.Value, err)
	}
	*n = Num(f)
	return nil
}

// Response is a parsed FlexQueryResponse (one statement per account).
type Response struct {
	QueryName  string      `xml:"queryName,attr"`
	Type       string      `xml:"type,attr"`
	Statements []Statement `xml:"FlexStatements>FlexStatement"`
}

// Statement is one account's Flex statement. Sections absent from the query are empty.
type Statement struct {
	AccountID     string               `xml:"accountId,attr"`
	FromDate      string               `xml:"fromDate,attr"`
	ToDate        string               `xml:"toDate,attr"`
	Period        string               `xml:"period,attr"`
	WhenGenerated string               `xml:"whenGenerated,attr"`
	Positions     []Position           `xml:"OpenPositions>OpenPosition"`
	Trades        []Trade              `xml:"Trades>Trade"`
	ClosedLots    []Trade              `xml:"Trades>Lot"`
	Cash          []CashReportCurrency `xml:"CashReport>CashReportCurrency"`
	EquitySummary []EquitySummary      `xml:"EquitySummaryInBase>EquitySummaryByReportDateInBase"`
	ChangeInNAV   *ChangeInNAV         `xml:"ChangeInNAV"`
	CashTxns      []CashTransaction    `xml:"CashTransactions>CashTransaction"`
}

// Position is an OpenPositions row. LevelOfDetail is SUMMARY or LOT.
type Position struct {
	AccountID      string `xml:"accountId,attr"`
	Currency       string `xml:"currency,attr"`
	FXRateToBase   Num    `xml:"fxRateToBase,attr"`
	AssetCategory  string `xml:"assetCategory,attr"`
	Symbol         string `xml:"symbol,attr"`
	Description    string `xml:"description,attr"`
	Conid          string `xml:"conid,attr"`
	ISIN           string `xml:"isin,attr"`
	Exchange       string `xml:"listingExchange,attr"`
	Multiplier     Num    `xml:"multiplier,attr"`
	ReportDate     string `xml:"reportDate,attr"`
	Quantity       Num    `xml:"position,attr"`
	MarkPrice      Num    `xml:"markPrice,attr"`
	MarketValue    Num    `xml:"positionValue,attr"`
	OpenPrice      Num    `xml:"openPrice,attr"`
	CostBasisPrice Num    `xml:"costBasisPrice,attr"`
	CostBasis      Num    `xml:"costBasisMoney,attr"`
	PercentOfNAV   Num    `xml:"percentOfNAV,attr"`
	UnrealizedPnL  Num    `xml:"fifoPnlUnrealized,attr"`
	Side           string `xml:"side,attr"`
	LevelOfDetail  string `xml:"levelOfDetail,attr"`
	// Set on LOT rows only.
	OpenDateTime          string `xml:"openDateTime,attr"`
	HoldingPeriodDateTime string `xml:"holdingPeriodDateTime,attr"`
	OriginatingOrderID    string `xml:"originatingOrderID,attr"`
	OriginatingTxnID      string `xml:"originatingTransactionID,attr"`
}

// Trade is a Trades row. LevelOfDetail EXECUTION marks a single fill. CLOSED_LOT rows
// (Statement.ClosedLots) are the lots a closing fill consumed, with their OpenDateTime.
type Trade struct {
	AccountID        string `xml:"accountId,attr"`
	Currency         string `xml:"currency,attr"`
	FXRateToBase     Num    `xml:"fxRateToBase,attr"`
	AssetCategory    string `xml:"assetCategory,attr"`
	Symbol           string `xml:"symbol,attr"`
	Description      string `xml:"description,attr"`
	Conid            string `xml:"conid,attr"`
	ISIN             string `xml:"isin,attr"`
	TradeID          string `xml:"tradeID,attr"`
	TransactionID    string `xml:"transactionID,attr"`
	ExecID           string `xml:"ibExecID,attr"`
	OrderID          string `xml:"ibOrderID,attr"`
	TradeDate        string `xml:"tradeDate,attr"`
	DateTime         string `xml:"dateTime,attr"`
	SettleDate       string `xml:"settleDateTarget,attr"`
	Exchange         string `xml:"exchange,attr"`
	BuySell          string `xml:"buySell,attr"`
	OpenClose        string `xml:"openCloseIndicator,attr"`
	Quantity         Num    `xml:"quantity,attr"`
	Price            Num    `xml:"tradePrice,attr"`
	TradeMoney       Num    `xml:"tradeMoney,attr"`
	Proceeds         Num    `xml:"proceeds,attr"`
	Commission       Num    `xml:"ibCommission,attr"`
	CommissionCcy    string `xml:"ibCommissionCurrency,attr"`
	NetCash          Num    `xml:"netCash,attr"`
	CostBasis        Num    `xml:"cost,attr"`
	RealizedPnL      Num    `xml:"fifoPnlRealized,attr"`
	MTMPnL           Num    `xml:"mtmPnl,attr"`
	OpenDateTime     string `xml:"openDateTime,attr"`
	HoldingPeriod    string `xml:"holdingPeriodDateTime,attr"`
	WhenRealized     string `xml:"whenRealized,attr"`
	LevelOfDetail    string `xml:"levelOfDetail,attr"`
	TransactionType  string `xml:"transactionType,attr"`
	Multiplier       Num    `xml:"multiplier,attr"`
	UnderlyingSymbol string `xml:"underlyingSymbol,attr"`
	UnderlyingConid  string `xml:"underlyingConid,attr"`
}

// CashReportCurrency is a CashReport row. Currency BASE_SUMMARY is the base-currency total.
type CashReportCurrency struct {
	AccountID          string `xml:"accountId,attr"`
	Currency           string `xml:"currency,attr"`
	LevelOfDetail      string `xml:"levelOfDetail,attr"`
	FromDate           string `xml:"fromDate,attr"`
	ToDate             string `xml:"toDate,attr"`
	StartingCash       Num    `xml:"startingCash,attr"`
	EndingCash         Num    `xml:"endingCash,attr"`
	EndingSettledCash  Num    `xml:"endingSettledCash,attr"`
	Deposits           Num    `xml:"deposits,attr"`
	Withdrawals        Num    `xml:"withdrawals,attr"`
	Dividends          Num    `xml:"dividends,attr"`
	Interest           Num    `xml:"brokerInterest,attr"`
	Commissions        Num    `xml:"commissions,attr"`
	WithholdingTax     Num    `xml:"withholdingTax,attr"`
	NetTradesSales     Num    `xml:"netTradesSales,attr"`
	NetTradesPurchases Num    `xml:"netTradesPurchases,attr"`
}

// EquitySummary is an EquitySummaryByReportDateInBase row: NAV on ReportDate in base currency.
type EquitySummary struct {
	AccountID        string `xml:"accountId,attr"`
	Currency         string `xml:"currency,attr"`
	ReportDate       string `xml:"reportDate,attr"`
	Cash             Num    `xml:"cash,attr"`
	Stock            Num    `xml:"stock,attr"`
	Options          Num    `xml:"options,attr"`
	Bonds            Num    `xml:"bonds,attr"`
	Funds            Num    `xml:"funds,attr"`
	InterestAccruals Num    `xml:"interestAccruals,attr"`
	DividendAccruals Num    `xml:"dividendAccruals,attr"`
	Total            Num    `xml:"total,attr"`
	TotalLong        Num    `xml:"totalLong,attr"`
	TotalShort       Num    `xml:"totalShort,attr"`
}

// ChangeInNAV is the period NAV bridge in base currency. TWR is a percent.
type ChangeInNAV struct {
	AccountID           string `xml:"accountId,attr"`
	Currency            string `xml:"currency,attr"`
	FromDate            string `xml:"fromDate,attr"`
	ToDate              string `xml:"toDate,attr"`
	StartingValue       Num    `xml:"startingValue,attr"`
	MTM                 Num    `xml:"mtm,attr"`
	Realized            Num    `xml:"realized,attr"`
	ChangeInUnrealized  Num    `xml:"changeInUnrealized,attr"`
	DepositsWithdrawals Num    `xml:"depositsWithdrawals,attr"`
	Dividends           Num    `xml:"dividends,attr"`
	WithholdingTax      Num    `xml:"withholdingTax,attr"`
	ChangeInDivAccruals Num    `xml:"changeInDividendAccruals,attr"`
	Interest            Num    `xml:"interest,attr"`
	ChangeInIntAccruals Num    `xml:"changeInInterestAccruals,attr"`
	Commissions         Num    `xml:"commissions,attr"`
	OtherFees           Num    `xml:"otherFees,attr"`
	EndingValue         Num    `xml:"endingValue,attr"`
	TWR                 Num    `xml:"twr,attr"`
}

// CashTransaction.Type values as IBKR spells them.
const (
	CashDividends          = "Dividends"
	CashPaymentInLieu      = "Payment In Lieu Of Dividends"
	CashWithholdingTax     = "Withholding Tax"
	CashInterestPaid       = "Broker Interest Paid"
	CashInterestReceived   = "Broker Interest Received"
	CashDepositsWithdrawal = "Deposits & Withdrawals"
	CashOtherFees          = "Other Fees"
)

// CashTransaction is a CashTransactions row (dividends, withholding, interest, deposits, fees).
type CashTransaction struct {
	AccountID     string `xml:"accountId,attr"`
	Currency      string `xml:"currency,attr"`
	FXRateToBase  Num    `xml:"fxRateToBase,attr"`
	AssetCategory string `xml:"assetCategory,attr"`
	Symbol        string `xml:"symbol,attr"`
	Description   string `xml:"description,attr"`
	Conid         string `xml:"conid,attr"`
	ISIN          string `xml:"isin,attr"`
	Type          string `xml:"type,attr"`
	Amount        Num    `xml:"amount,attr"`
	DateTime      string `xml:"dateTime,attr"`
	ReportDate    string `xml:"reportDate,attr"`
	SettleDate    string `xml:"settleDate,attr"`
	TransactionID string `xml:"transactionID,attr"`
	ActionID      string `xml:"actionID,attr"`
	LevelOfDetail string `xml:"levelOfDetail,attr"`
}

// Parse decodes a FlexQueryResponse document. A FlexStatementResponse error
// document is returned as *Error.
func Parse(b []byte) (*Response, error) {
	var root struct {
		XMLName xml.Name
		statusResp
		Response
	}
	if err := xml.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("flex parse: %w", err)
	}
	switch root.XMLName.Local {
	case "FlexQueryResponse":
		return &root.Response, nil
	case "FlexStatementResponse":
		return nil, &Error{Code: root.ErrorCode, Message: root.ErrorMessage}
	}
	return nil, fmt.Errorf("flex parse: unexpected root <%s>", root.XMLName.Local)
}
