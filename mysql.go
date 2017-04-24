package yiigo

import (
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type MySQL struct {
	DB    string
	Table string
}

var dbmap map[string]*sqlx.DB

// SQL expression
type expr struct {
	expr string
	args []interface{}
}

/**
 * 初始化DB
 * @param dbnames ...string 数据库配置名称
 */
func InitDB(dbnames ...string) {
	if len(dbnames) == 0 {
		dbnames = append(dbnames, "db")
	}

	dbmap = make(map[string]*sqlx.DB)

	for _, v := range dbnames {
		host := GetEnvString(v, "host", "localhost")
		port := GetEnvInt(v, "port", 3306)
		username := GetEnvString(v, "username", "root")
		password := GetEnvString(v, "password", "")
		database := GetEnvString(v, "database", "test")
		charset := GetEnvString(v, "charset", "utf8mb4")
		collection := GetEnvString(v, "collection", "utf8_general_ci")

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&collation=%s&parseTime=True&loc=Local", username, password, host, port, database, charset, collection)
		db, err := sqlx.Open("mysql", dsn)

		if err != nil {
			LogError("[MySQL] Connect Error: ", err.Error())
			panic(err)
		}

		db.SetMaxOpenConns(GetEnvInt("db", "maxOpenConns", 20))
		db.SetMaxIdleConns(GetEnvInt("db", "maxIdleConns", 10))

		err = db.Ping()

		if err != nil {
			db.Close()
		}

		dbmap[v] = db
	}
}

/**
 * 获取db
 * @return *sqlx.DB
 */
func (m *MySQL) getDB() *sqlx.DB {
	dbname := m.DB

	if dbname == "" {
		dbname = "db"
	}

	db, ok := dbmap[dbname]

	if !ok {
		LogErrorf("[MySQL] Database Error: %s is not initialized", m.DB)
		panic(fmt.Sprintf("mysql error: database %s is not initialized", m.DB))
	}

	return db
}

/**
 * 获取表前缀
 * @return string
 */
func (m *MySQL) getPrefix() string {
	dbname := m.DB

	if dbname == "" {
		dbname = "db"
	}

	prefix := GetEnvString(dbname, "prefix", "")

	return prefix
}

/**
 * Insert 插入
 * @param data X 插入数据
 * @return int64, error 新增记录ID
 */
func (m *MySQL) Insert(data X) (int64, error) {
	db := m.getDB()

	sql, binds := m.buildInsert(data)
	result, err := db.Exec(sql, binds...)

	if err != nil {
		LogError("[MySQL] Insert Error: ", err.Error())
		return 0, err
	}

	id, _ := result.LastInsertId()

	return id, nil
}

/**
 * BatchInsert 批量插入
 * @param columns []string 插入的字段
 * @param data []X 插入数据
 * @return int64, error 影响的行数
 */
func (m *MySQL) BatchInsert(columns []string, data []X) (int64, error) {
	db := m.getDB()

	sql, binds := m.buildBatchInsert(columns, data)
	result, err := db.Exec(sql, binds...)

	if err != nil {
		LogError("[MySQL] BatchInsert Error: ", err.Error())
		return 0, err
	}

	rows, _ := result.RowsAffected()

	return rows, nil
}

/**
 * Update 更新
 * @param query X 查询条件
 * yiigo.X{
 *     where string WHERE条件语句
 *     binds []interface{} WHERE语句中 "?" 的绑定值
 * }
 * @param data X 更新字段
 * @return int64, error 影响的行数
 */
func (m *MySQL) Update(query X, data X) (int64, error) {
	db := m.getDB()

	sql, binds := m.buildUpdate(query, data)
	_sql, args, _ := sqlx.In(sql, binds...)
	result, err := db.Exec(_sql, args...)

	if err != nil {
		LogError("[MySQL] Update Error: ", err.Error())
		return 0, err
	}

	rows, _ := result.RowsAffected()

	return rows, nil
}

/**
 * Count 获取记录数
 * @param query X 查询条件
 * yiigo.X{
 *     where string WHERE语句
 *     binds []interface{} WHERE语句中 "?" 的绑定值
 * }
 * @param data *int 查询数据
 * @param columns ...string 聚合字段，默认为：*
 * @return error
 */
func (m *MySQL) Count(query X, data *int, columns ...string) error {
	db := m.getDB()

	if len(columns) > 0 {
		query["select"] = fmt.Sprintf("COUNT(%s)", columns[0])
	} else {
		query["select"] = "COUNT(*)"
	}

	count := 0

	sql, binds := m.buildQuery(query)
	_sql, args, _ := sqlx.In(sql, binds...)
	err := db.Get(&count, _sql, args...)

	*data = count

	return err
}

/**
 * FindOne 查询单条记录
 * @param query X 查询条件
 * yiigo.X{
 *     select string SELECT语句
 *     join string JOIN语句
 *     where string WHERE语句
 *     binds []interface{} WHERE语句中 "?" 的绑定值
 * }
 * @param data interface{} 查询数据 (struct指针)
 * @return error
 */
func (m *MySQL) FindOne(query X, data interface{}) error {
	db := m.getDB()

	query["limit"] = 1

	sql, binds := m.buildQuery(query)
	_sql, args, _ := sqlx.In(sql, binds...)
	err := db.Get(data, _sql, args...)

	if err != nil {
		msg := err.Error()

		if msg != "sql: no rows in result set" {
			LogError("[MySQL] FindOne Error: ", msg)
		}

		return err
	}

	return nil
}

/**
 * Find 查询多条记录
 * @param query X 查询条件
 * yiigo.X{
 *     select string SELECT语句
 *     join []string JOIN语句
 *     where string WHERE语句
 *     group string GROUP BY语句
 *     order string ORDER BY语句
 *     offset int OFFSET语句
 *     limit int LIMIT语句
 *     binds []interface{} WHERE语句中 "?" 的绑定值
 * }
 * @param data interface{} 查询数据 (struct切片指针)
 * @return error
 */
func (m *MySQL) Find(query X, data interface{}) error {
	db := m.getDB()

	sql, binds := m.buildQuery(query)
	_sql, args, _ := sqlx.In(sql, binds...)
	err := db.Select(data, _sql, args...)

	if err != nil {
		msg := err.Error()

		if msg != "sql: no rows in result set" {
			LogError("[MySQL] Find Error: ", msg)
		}

		return err
	}

	return nil
}

/**
 * FindAll 查询所有记录
 * @param data interface{} 查询数据 (struct切片指针)
 * @param columns ...string 查询字段
 * @return error
 */
func (m *MySQL) FindAll(data interface{}, columns ...string) error {
	db := m.getDB()

	query := X{}

	if len(columns) > 0 {
		query["select"] = strings.Join(columns, ",")
	}

	sql, binds := m.buildQuery(query)
	err := db.Select(data, sql, binds...)

	if err != nil {
		msg := err.Error()

		if msg != "sql: no rows in result set" {
			LogError("[MySQL] FindAll Error: ", msg)
		}

		return err
	}

	return nil
}

/**
 * Delete 删除
 * @param query X 查询条件
 * yiigo.X{
 *     where string WHERE语句
 *     binds []interface{} WHERE语句中 "?" 的绑定值
 * }
 * @return int64, error 影响的行数
 */
func (m *MySQL) Delete(query X) (int64, error) {
	db := m.getDB()

	sql, binds := m.buildDelete(query)
	_sql, args, _ := sqlx.In(sql, binds...)
	result, err := db.Exec(_sql, args...)

	if err != nil {
		LogError("[MySQL] Delete Error: ", err.Error())
		return 0, err
	}

	rows, _ := result.RowsAffected()

	return rows, nil
}

/**
 * DoTransactions 事务处理
 * @param operations X 操作集合
 * yiigo.X{
 *     "insert": yiigo.X{
 *	 		"table": string,
 *			"data": yiigo.X,
 *     }
 *     "batchInsert": yiigo.X{
 *	 		"table": string,
 *			"columns": []string
 *			"data": []yiigo.X,
 *     }
 *     "update": yiigo.X{
 *	 		"query": yiigo.X{
 *	 			"table": string,
 * 				"where": string,
 *				"binds": []interface{},
 *          },
 *			"data": yiigo.X,
 *     }
 *	   "delete": yiigo.X{
 *	 		"table": string,
 * 			"where": string,
 *			"binds": []interface{},
 *     }
 * }
 * @return error
 */
func (m *MySQL) DoTransactions(operations X) error {
	db := m.getDB()
	tx, err := db.Begin()

	if err != nil {
		LogError("[MySQL] DoTransactions Error: ", err.Error())
		return err
	}

	for key, value := range operations {
		opt := value.(X)

		switch key {
		case "insert":
			table := []string{}
			data := X{}

			if v, ok := opt["table"]; ok {
				table = append(table, v.(string))
			}

			if v, ok := opt["data"]; ok {
				data = v.(X)
			}

			sql, binds := m.buildInsert(data, table...)
			_, err = tx.Exec(sql, binds...)

			if err != nil {
				break
			}
		case "batchInsert":
			table := []string{}
			columns := []string{}
			data := []X{}

			if v, ok := opt["table"]; ok {
				table = append(table, v.(string))
			}

			if v, ok := opt["columns"]; ok {
				columns = v.([]string)
			}

			if v, ok := opt["data"]; ok {
				data = v.([]X)
			}

			sql, binds := m.buildBatchInsert(columns, data, table...)
			_, err = tx.Exec(sql, binds...)

			if err != nil {
				break
			}

			if err != nil {
				break
			}
		case "update":
			fmt.Println("执行更新")
			query := X{}
			data := X{}

			if v, ok := opt["query"]; ok {
				query = v.(X)
			}

			if v, ok := opt["data"]; ok {
				data = v.(X)
			}

			sql, binds := m.buildUpdate(query, data)
			_sql, args, _ := sqlx.In(sql, binds...)
			_, err = tx.Exec(_sql, args...)

			if err != nil {
				break
			}
		case "delete":
			sql, binds := m.buildDelete(opt)
			_sql, args, _ := sqlx.In(sql, binds...)
			_, err = tx.Exec(_sql, args...)

			if err != nil {
				break
			}
		}

		if err != nil {
			break
		}
	}

	if err != nil {
		tx.Rollback()
		LogError("[MySQL] DoTransactions Error: ", err.Error())

		return err
	}

	tx.Commit()

	return nil
}

/**
 * Expr SQL表达式
 * @param expression string 表达式，如：yiigo.Expr("price * ? + ?", 2, 100)
 * @param args ...interface{} 表达式中 "?" 的绑定值
 * @return *expr
 */
func Expr(expression string, args ...interface{}) *expr {
	return &expr{expr: expression, args: args}
}

/**
 * buildInsert 构建插入SQL
 * @param data X 插入数据
 * @param tables ...string 插入表
 * @return string, []interface{}
 */
func (m *MySQL) buildInsert(data X, tables ...string) (string, []interface{}) {
	if len(tables) == 0 {
		tables = append(tables, m.Table)
	}

	prefix := m.getPrefix()

	columns := []string{}
	placeholders := []string{}
	binds := []interface{}{}

	for k, v := range data {
		columns = append(columns, k)
		placeholders = append(placeholders, "?")
		binds = append(binds, v)
	}

	sql := fmt.Sprintf("INSERT INTO %s%s (%s) VALUES (%s)", prefix, tables[0], strings.Join(columns, ","), strings.Join(placeholders, ","))

	return sql, binds
}

/**
 * buildBatchInsert 构建批量插入SQL
 * @param columns []string 插入字段
 * @param data []X 插入数据
 * @param tables ...string 插入表
 * @return string, []interface{}
 */
func (m *MySQL) buildBatchInsert(columns []string, data []X, tables ...string) (string, []interface{}) {
	if len(tables) == 0 {
		tables = append(tables, m.Table)
	}

	prefix := m.getPrefix()

	placeholders := []string{}
	binds := []interface{}{}

	for _, v := range data {
		bindvars := []string{}

		for _, column := range columns {
			binds = append(binds, v[column])
			bindvars = append(bindvars, "?")
		}

		placeholders = append(placeholders, fmt.Sprintf("(%s)", strings.Join(bindvars, ",")))
	}

	sql := fmt.Sprintf("INSERT INTO %s%s (%s) VALUES %s", prefix, tables[0], strings.Join(columns, ","), strings.Join(placeholders, ","))

	return sql, binds
}

/**
 * buildUpdate 构建更新SQL
 * @param query X 查询条件
 * @param data X 更新数据
 * @return string, []interface{}
 */
func (m *MySQL) buildUpdate(query X, data X) (string, []interface{}) {
	table := m.Table
	prefix := m.getPrefix()

	clauses := []string{}
	set := []string{}
	binds := []interface{}{}

	if v, ok := query["table"]; ok {
		table = v.(string)
	}

	clauses = append(clauses, fmt.Sprintf("UPDATE %s%s", prefix, table))

	for k, v := range data {
		if expr, ok := v.(*expr); ok {
			set = append(set, fmt.Sprintf("%s = %s", k, expr.expr))
			binds = append(binds, expr.args...)
		} else {
			set = append(set, fmt.Sprintf("%s = ?", k))
			binds = append(binds, v)
		}
	}

	clauses = append(clauses, fmt.Sprintf("SET %s", strings.Join(set, ",")))

	if v, ok := query["where"]; ok {
		clauses = append(clauses, fmt.Sprintf("WHERE %s", v.(string)))
	}

	if v, ok := query["binds"]; ok {
		binds = append(binds, v.([]interface{})...)
	}

	sql := strings.Join(clauses, " ")
	fmt.Println("[sql]", sql)
	fmt.Println("[binds]", binds)
	return sql, binds
}

/**
 * buildQuery 构建查询SQL
 * @param query X 查询条件
 * @return string, []interface{}
 */
func (m *MySQL) buildQuery(query X) (string, []interface{}) {
	table := m.Table
	prefix := m.getPrefix()

	clauses := []string{}
	binds := []interface{}{}

	if v, ok := query["select"]; ok {
		clauses = append(clauses, fmt.Sprintf("SELECT %s", v.(string)))
	} else {
		clauses = append(clauses, "SELECT *")
	}

	if v, ok := query["table"]; ok {
		table = v.(string)
	}

	if v, ok := query["join"]; ok {
		clauses = append(clauses, fmt.Sprintf("FROM %s%s AS a", prefix, table))

		for _, join := range v.([]string) {
			clauses = append(clauses, join)
		}
	} else {
		clauses = append(clauses, fmt.Sprintf("FROM %s%s", prefix, table))
	}

	if v, ok := query["where"]; ok {
		clauses = append(clauses, fmt.Sprintf("WHERE %s", v.(string)))
	}

	if v, ok := query["group"]; ok {
		clauses = append(clauses, fmt.Sprintf("GROUP BY %s", v.(string)))
	}

	if v, ok := query["order"]; ok {
		clauses = append(clauses, fmt.Sprintf("ORDER BY %s", v.(string)))
	}

	if v, ok := query["offset"]; ok {
		clauses = append(clauses, fmt.Sprintf("OFFSET %d", v.(int)))
	}

	if v, ok := query["limit"]; ok {
		clauses = append(clauses, fmt.Sprintf("LIMIT %d", v.(int)))
	}

	if v, ok := query["binds"]; ok {
		binds = append(binds, v.([]interface{})...)
	}

	sql := strings.Join(clauses, " ")

	return sql, binds
}

/**
 * buildDelete 构建删除SQL
 * @param query X 查询条件
 * @return string, []interface{}
 */
func (m *MySQL) buildDelete(query X) (string, []interface{}) {
	table := m.Table
	prefix := m.getPrefix()

	clauses := []string{}
	binds := []interface{}{}

	if v, ok := query["table"]; ok {
		table = v.(string)
	}

	clauses = append(clauses, fmt.Sprintf("DELETE FROM %s%s", prefix, table))

	if v, ok := query["where"]; ok {
		clauses = append(clauses, fmt.Sprintf("WHERE %s", v.(string)))
	}

	if v, ok := query["binds"]; ok {
		binds = append(binds, v.([]interface{})...)
	}

	sql := strings.Join(clauses, " ")

	return sql, binds
}
