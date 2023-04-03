import os

import tomlkit
import yaml

here = os.path.abspath(os.path.dirname(__file__))

for filename in os.listdir(os.path.join(here, 'content/posts')):
    print(f"process '{filename}':")
    if filename.endswith(".md"):
        filepath = os.path.join(here, 'content/posts', filename)
        with open(filepath, 'r') as file:
            file_content = file.read()
        flag = '---\n'
        yaml_end_index = file_content.find(flag, len(flag))
        if yaml_end_index == -1:
            continue
        yaml_block = file_content[len(flag):yaml_end_index]

        metadata_dict = yaml.safe_load(yaml_block)
        toml_block = tomlkit.dumps(metadata_dict)
        toml_block = toml_block.splitlines()
        tags_index = [i for i, s in enumerate(toml_block) if 'tags' in s]
        if tags_index:
            toml_block.insert(tags_index[0], '[taxonomies]')
        toml_block = "\n".join(toml_block)
        print(toml_block)

        new_file_content = file_content.replace(f"---\n{yaml_block}---\n", '+++\n' + toml_block + '\n+++\n')
        with open(filepath, 'w') as file:
            file.write(new_file_content)

        print("\n")
