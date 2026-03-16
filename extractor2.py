import re
import os
import string

def sanitize_filename(name):
    # Keep only normal characters
    valid_chars = "-_.() %s%s" % (string.ascii_letters, string.digits)
    # allow slashes too
    valid_chars += "/"
    return ''.join(c for c in name if c in valid_chars).strip()

def process_file(source_file, pattern):
    with open(source_file, 'r', encoding='utf-8') as f:
        text = f.read()

    pieces = re.split(pattern, text, flags=re.MULTILINE)

    for i in range(1, len(pieces), 2):
        filename = pieces[i]
        content = pieces[i+1]
        
        filename = sanitize_filename(filename)
        
        # fix: some names might have things like "(UPDATE IN manager_enhanced.go)"
        if '(' in filename:
            filename = filename.split('(')[0].strip()
            
        print(f"Writing {filename}...")
        
        lines = content.split('\n')
        # strip equals and dashes
        while lines and (lines[0].startswith('===') or lines[0].startswith('---') or not lines[0].strip()):
            lines.pop(0)

        while lines and (lines[-1].startswith('===') or lines[-1].startswith('---') or not lines[-1].strip()):
            lines.pop()
            
        final_content = '\n'.join(lines) + '\n'
        
        os.makedirs(os.path.dirname(filename) or '.', exist_ok=True)
        with open(filename, 'w', encoding='utf-8') as f:
            f.write(final_content)

process_file('InstDep.txt', r'^FILE\s*(?:\d+)?:\s*(.*?)$')
process_file('v2.txt', r'^FILE:\s*(.*?)$')
