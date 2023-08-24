# andy
A tool to execute SQL scripts in a specific order.

Now MS-SQL only.

## Basic Usage

**Dry Run Mode** - show what scripts will be executed. 

  go build andy.go && git diff --name-only tag-1 tag-42 | grep -vE '^\.' | grep -vE '/$' | ./andy -i --dry-run

**Real mode**  

  go build andy.go && git diff --name-only tag-1 tag-42 | grep -vE '^\.' | grep -vE '/$' | ./andy -i --dbstring=sqlserver://user:password@dbname:1433?database=master 
