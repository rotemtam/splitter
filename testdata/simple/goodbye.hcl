schema "goodbye" {
}
table "transactions" {
  schema = schema.goodbye
  column "id" {
    type = int
  }
  column "user_id" {
    type = int
  }
  column "amount" {
    type = decimal
  }
  column "is_income" {
    type = boolean
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "user_fk" {
    on_update   = NO_ACTION
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_delete   = CASCADE
  }
}
