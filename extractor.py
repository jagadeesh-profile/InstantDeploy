import os
import re

def extract(filename, is_v1):
    with open(filename, 'r', encoding='utf-8', errors='ignore') as f:
        lines = f.read().splitlines()
    
    current_file = None
    current_content = []
    
    if is_v1: # InstDep.txt
        for line in lines:
            m = re.match(r'^FILE\s+\d+:\s+(.*)', line)
            if m:
                if current_file:
                    # save previous file, stripping top/bottom ===
                    write_file(current_file, current_content, True)
                current_file = m.group(1).strip()
                current_content = []
            elif current_file is not None:
                current_content.append(line)
        
        if current_file:
            write_file(current_file, current_content, True)
            
    else: # v2.txt
        for line in lines:
            m = re.match(r'^FILE:\s+(.*)', line)
            if m:
                if current_file:
                    write_file(current_file, current_content, False)
                current_file = m.group(1).strip()
                current_content = []
            elif current_file is not None:
                current_content.append(line)
                
        if current_file:
            write_file(current_file, current_content, False)

def write_file(filepath, content, is_v1):
    if is_v1:
        # Ignore lines starting with === until first non-===
        while content and content[0].startswith('===='):
            content.pop(0)
    else:
        # Ignore lines starting with ---
        while content and content[0].startswith('---'):
            content.pop(0)
            
    # Also drop last few lines if they are '====' in v1
    if is_v1:
        while content and content[-1].startswith('===='):
            content.pop()

    # Drop empty lines at top
    while content and not content[0].strip():
        content.pop(0)
        
    os.makedirs(os.path.dirname(filepath) or '.', exist_ok=True)
    with open(filepath, 'w', encoding='utf-8') as f:
        f.write('\n'.join(content) + '\n')
    print(f"Written: {filepath}")

if __name__ == '__main__':
    extract('InstDep.txt', True)
    extract('v2.txt', False)
