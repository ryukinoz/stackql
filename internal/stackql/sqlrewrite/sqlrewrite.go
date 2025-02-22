package sqlrewrite

import (
	"github.com/stackql/stackql/internal/stackql/drm"
	"github.com/stackql/stackql/internal/stackql/internal_data_transfer/internal_relational_dto"
	"github.com/stackql/stackql/internal/stackql/internal_data_transfer/internaldto"
	"github.com/stackql/stackql/internal/stackql/internal_data_transfer/relationaldto"
	"github.com/stackql/stackql/internal/stackql/tableinsertioncontainer"
	"github.com/stackql/stackql/internal/stackql/tablenamespace"
	"github.com/stackql/stackql/internal/stackql/taxonomy"
)

type SQLRewriteInput interface {
	GetNamespaceCollection() tablenamespace.TableNamespaceCollection
	GetDRMConfig() drm.DRMConfig
	GetColumnDescriptors() []relationaldto.RelationalColumn
	GetBaseControlCounters() internaldto.TxnControlCounters
	GetFromString() string
	GetIndirectContexts() []drm.PreparedStatementCtx
	GetSelectSuffix() string
	GetRewrittenWhere() string
	GetSecondaryCtrlCounters() []internaldto.TxnControlCounters
	GetTables() taxonomy.TblMap
	GetTableInsertionContainers() []tableinsertioncontainer.TableInsertionContainer
	WithIndirectContexts(indirectContexts []drm.PreparedStatementCtx) SQLRewriteInput
}

type StandardSQLRewriteInput struct {
	dc                       drm.DRMConfig
	columnDescriptors        []relationaldto.RelationalColumn
	baseControlCounters      internaldto.TxnControlCounters
	selectSuffix             string
	rewrittenWhere           string
	secondaryCtrlCounters    []internaldto.TxnControlCounters
	tables                   taxonomy.TblMap
	fromString               string
	tableInsertionContainers []tableinsertioncontainer.TableInsertionContainer
	namespaceCollection      tablenamespace.TableNamespaceCollection
	indirectContexts         []drm.PreparedStatementCtx
}

func NewStandardSQLRewriteInput(
	dc drm.DRMConfig,
	columnDescriptors []relationaldto.RelationalColumn,
	baseControlCounters internaldto.TxnControlCounters,
	selectSuffix string,
	rewrittenWhere string,
	secondaryCtrlCounters []internaldto.TxnControlCounters,
	tables taxonomy.TblMap,
	fromString string,
	tableInsertionContainers []tableinsertioncontainer.TableInsertionContainer,
	namespaceCollection tablenamespace.TableNamespaceCollection,
) SQLRewriteInput {
	return &StandardSQLRewriteInput{
		dc:                       dc,
		columnDescriptors:        columnDescriptors,
		baseControlCounters:      baseControlCounters,
		selectSuffix:             selectSuffix,
		rewrittenWhere:           rewrittenWhere,
		secondaryCtrlCounters:    secondaryCtrlCounters,
		tables:                   tables,
		fromString:               fromString,
		tableInsertionContainers: tableInsertionContainers,
		namespaceCollection:      namespaceCollection,
	}
}

func (ri *StandardSQLRewriteInput) GetDRMConfig() drm.DRMConfig {
	return ri.dc
}

func (ri *StandardSQLRewriteInput) WithIndirectContexts(indirectContexts []drm.PreparedStatementCtx) SQLRewriteInput {
	ri.indirectContexts = indirectContexts
	return ri
}

func (ri *StandardSQLRewriteInput) GetIndirectContexts() []drm.PreparedStatementCtx {
	return ri.indirectContexts
}

func (ri *StandardSQLRewriteInput) GetNamespaceCollection() tablenamespace.TableNamespaceCollection {
	return ri.namespaceCollection
}

func (ri *StandardSQLRewriteInput) GetColumnDescriptors() []relationaldto.RelationalColumn {
	return ri.columnDescriptors
}

func (ri *StandardSQLRewriteInput) GetTableInsertionContainers() []tableinsertioncontainer.TableInsertionContainer {
	return ri.tableInsertionContainers
}

func (ri *StandardSQLRewriteInput) GetBaseControlCounters() internaldto.TxnControlCounters {
	return ri.baseControlCounters
}

func (ri *StandardSQLRewriteInput) GetSelectSuffix() string {
	return ri.selectSuffix
}

func (ri *StandardSQLRewriteInput) GetFromString() string {
	return ri.fromString
}

func (ri *StandardSQLRewriteInput) GetRewrittenWhere() string {
	return ri.rewrittenWhere
}

func (ri *StandardSQLRewriteInput) GetSecondaryCtrlCounters() []internaldto.TxnControlCounters {
	return ri.secondaryCtrlCounters
}

func (ri *StandardSQLRewriteInput) GetTables() taxonomy.TblMap {
	return ri.tables
}

func GenerateSelectDML(input SQLRewriteInput) (drm.PreparedStatementCtx, error) {
	dc := input.GetDRMConfig()
	cols := input.GetColumnDescriptors()
	var txnCtrlCtrs internaldto.TxnControlCounters
	var secondaryCtrlCounters []internaldto.TxnControlCounters
	selectSuffix := input.GetSelectSuffix()
	rewrittenWhere := input.GetRewrittenWhere()
	var columns []internaldto.ColumnMetadata
	var relationalColumns []relationaldto.RelationalColumn
	var tableAliases []string
	for _, col := range cols {
		relationalColumn := col
		columns = append(columns, internal_relational_dto.NewRelayedColDescriptor(relationalColumn, relationalColumn.GetType()))
		// TODO: Need a way to handle postgres differences. This is a fragile point
		relationalColumns = append(relationalColumns, relationalColumn)
	}
	genIdColName := dc.GetControlAttributes().GetControlGenIdColumnName()
	sessionIDColName := dc.GetControlAttributes().GetControlSsnIdColumnName()
	txnIdColName := dc.GetControlAttributes().GetControlTxnIdColumnName()
	insIdColName := dc.GetControlAttributes().GetControlInsIdColumnName()
	insEncodedColName := dc.GetControlAttributes().GetControlInsertEncodedIdColumnName()
	inputContainers := input.GetTableInsertionContainers()
	if len(inputContainers) > 0 {
		_, txnCtrlCtrs = inputContainers[0].GetTableTxnCounters()
	} else {
		txnCtrlCtrs = input.GetBaseControlCounters()
		secondaryCtrlCounters = input.GetSecondaryCtrlCounters()
	}
	i := 0
	for _, tb := range inputContainers {
		if i > 0 {
			_, secondaryCtr := tb.GetTableTxnCounters()
			secondaryCtrlCounters = append(secondaryCtrlCounters, secondaryCtr)
		}
		v := tb.GetTableMetadata()
		alias := v.GetAlias()
		tableAliases = append(tableAliases, alias)
		i++
	}

	query, err := dc.GetSQLSystem().ComposeSelectQuery(relationalColumns, tableAliases, input.GetFromString(), rewrittenWhere, selectSuffix)
	if err != nil {
		return nil, err
	}
	rv := drm.NewPreparedStatementCtx(
		query,
		"",
		genIdColName,
		sessionIDColName,
		nil,
		txnIdColName,
		insIdColName,
		insEncodedColName,
		columns,
		len(input.GetTables()),
		txnCtrlCtrs,
		secondaryCtrlCounters,
		input.GetDRMConfig().GetNamespaceCollection(),
		dc.GetSQLSystem(),
	)
	rv.SetIndirectContexts(input.GetIndirectContexts())
	return rv, nil
}
