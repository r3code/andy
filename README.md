# andy

A tool to execute SQL scripts in a specific order.

It executes scripst form folders in the order as specified, files sorted by name and executed.

Now MS-SQL Server only.

## Basic Usage

**Dry Run Mode** - show what scripts will be executed. 

    go build andy.go && git diff --name-only tag-1 tag-42 | grep -vE '^\.' | grep -vE '/$' | ./andy -i --dry-run

**Real mode**  

    go build andy.go && git diff --name-only tag-1 tag-42 | grep -vE '^\.' | grep -vE '/$' | ./andy -i --dbstring=sqlserver://user:password@dbname:1433?database=master 

## Manual testing 

Run with -i --dry-run input file names one per line:

    views/1.sql  
    views/2.sql  
    procs/p1.sql  

Press Ctrl+D 

Check the output.
