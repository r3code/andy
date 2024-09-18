cp ./config.yml.example ./config.yml
cat bad_scripts.txt | ./andy -i --dir=./databases/ --config=./config.yml
