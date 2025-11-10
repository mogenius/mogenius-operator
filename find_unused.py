import os
import re
from typing import List, Set

def get_go_files(directory: str) -> List[str]:
    """Recursively gather all .go files in the specified directory."""
    go_files = []
    for root, _, files in os.walk(directory):
        for file in files:
            if file.endswith('.go'):
                go_files.append(os.path.join(root, file))
    print(f"\nScanned directory: {directory}")
    print(f"Total Go files found: {len(go_files)}\n")
    return go_files

def extract_type_names(file_path: str) -> Set[str]:
    """Extract type names from a Go source file."""
    type_names = set()
    # Adjust regex for complex types, including interfaces and multi-line declarations
    type_decl = re.compile(r'\btype\s+([A-Z]\w*)\b', re.MULTILINE)

    with open(file_path, 'r', encoding='utf-8') as f:
        contents = f.read()
        matches = type_decl.findall(contents)
        type_names.update(matches)

    return type_names

def is_type_used(type_name: str, files: List[str], def_file: str, def_lines: Set[int]) -> bool:
    occurrence = re.compile(r'\b' + re.escape(type_name) + r'\b')
    for file_path in files:
        with open(file_path, 'r', encoding='utf-8') as f:
            lines = f.readlines()
            for idx, line in enumerate(lines):
                if file_path == def_file and idx in def_lines:
                    continue  # skip the definition line(s)
                if occurrence.search(line):
                    return True
    return False

def extract_type_names_and_lines(file_path: str):
    """Extract type names and their line numbers from a Go source file."""
    type_names = set()
    type_lines = dict()
    type_decl = re.compile(r'\btype\s+([A-Z]\w*)\b')
    with open(file_path, 'r', encoding='utf-8') as f:
        for idx, line in enumerate(f):
            match = type_decl.search(line)
            if match:
                name = match.group(1)
                type_names.add(name)
                if name not in type_lines:
                    type_lines[name] = set()
                type_lines[name].add(idx)
    return type_names, type_lines

def find_unused_types(directory: str):
    """Find and report unused type names in the Go project directory."""
    go_files = [f for f in get_go_files(directory) if not f.endswith('_test.go')]
    all_types = set()
    types_per_file = {}
    type_lines_per_file = {}

    for file in go_files:
        types_in_file, type_lines = extract_type_names_and_lines(file)
        if types_in_file:
            types_per_file[file] = types_in_file
            type_lines_per_file[file] = type_lines
            all_types.update(types_in_file)
        print(f"File: {os.path.relpath(file)}, Types found: {types_in_file}")

    print(f"\nTotal unique type names extracted: {len(all_types)}\n")

    unused_types = []
    for file, types in types_per_file.items():
        for type_name in types:
            def_lines = type_lines_per_file[file][type_name]
            if not is_type_used(type_name, go_files, file, def_lines):
                unused_types.append(type_name)

    if unused_types:
        print("Unused types detected:")
        for type_ in unused_types:
            print(f"  - {type_}")
    else:
        print("No unused types found.\n")

def extract_func_names_and_lines(file_path: str):
    """Extract function names and their line numbers from a Go source file."""
    func_names = set()
    func_lines = dict()
    # This regex matches both regular and method functions (with receiver)
    func_decl = re.compile(r'^\s*func\s+(?:\([^)]+\)\s*)?([A-Za-z_]\w*)\b', re.MULTILINE)
    with open(file_path, 'r', encoding='utf-8') as f:
        for idx, line in enumerate(f):
            match = func_decl.match(line)
            if match:
                name = match.group(1)
                func_names.add(name)
                if name not in func_lines:
                    func_lines[name] = set()
                func_lines[name].add(idx)
    return func_names, func_lines

def is_func_used(func_name: str, files: List[str], def_file: str, def_lines: Set[int]) -> bool:
    occurrence = re.compile(r'\b' + re.escape(func_name) + r'\b')
    for file_path in files:
        with open(file_path, 'r', encoding='utf-8') as f:
            lines = f.readlines()
            for idx, line in enumerate(lines):
                if file_path == def_file and idx in def_lines:
                    continue  # skip the definition line(s)
                if occurrence.search(line):
                    return True
    return False

def find_unused_functions(directory: str):
    """Find and report unused function names in the Go project directory."""
    go_files = [f for f in get_go_files(directory) if not f.endswith('_test.go')]
    all_funcs = set()
    funcs_per_file = {}
    func_lines_per_file = {}

    for file in go_files:
        funcs_in_file, func_lines = extract_func_names_and_lines(file)
        if funcs_in_file:
            funcs_per_file[file] = funcs_in_file
            func_lines_per_file[file] = func_lines
            all_funcs.update(funcs_in_file)
        print(f"File: {os.path.relpath(file)}, Functions found: {funcs_in_file}")

    print(f"\nTotal unique function names extracted: {len(all_funcs)}\n")

    unused_funcs = []
    for file, funcs in funcs_per_file.items():
        for func_name in funcs:
            def_lines = func_lines_per_file[file][func_name]
            if not is_func_used(func_name, go_files, file, def_lines):
                # if the function is not used there should only be one line here
                # so we can just convert the set to a string directly and remove {} to get line number
                line_nums = str(def_lines).strip('{}')
                # we report file name along with line number for easier locating
                unused_funcs.append(f"{file}#{line_nums} - {func_name}")

    if unused_funcs:
        print("Unused functions detected:")
        for func_ in unused_funcs:
            print(f"  - {func_}")
    else:
        print("No unused functions found.\n")

if __name__ == "__main__":
    project_dir = '.'  # Enter your project's directory here
    find_unused_types(project_dir)
    find_unused_functions(project_dir)
