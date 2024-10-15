schema "hello" {
  comment = "This is the hello schema"
}
table "users" {
  schema = schema.hello
  column "id" {
    type = int
  }
  column "name" {
    type = text
  }
  primary_key {
    columns = [
      column.id
    ]
  }
}
