cp ./config.yml.example ./config.yml
cat good_scripts.txt | ./andy -i -dry-run --dir=./databases/ --config=./config.yml
