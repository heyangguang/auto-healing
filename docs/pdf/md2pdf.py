#!/usr/bin/env python3
"""Markdown to PDF: markdown -> HTML -> Chromium PDF via Playwright."""
import markdown
import os
import sys
from pathlib import Path

md_file = sys.argv[1] if len(sys.argv) > 1 else os.path.join(os.path.dirname(__file__), '..', 'system-features-guide.md')
md_file = os.path.abspath(md_file)
base_dir = Path(md_file).resolve().parent
pdf_name = os.path.basename(md_file).replace('.md', '.pdf')
pdf_file = os.path.join(os.path.dirname(os.path.abspath(__file__)), pdf_name)
html_file = pdf_file.replace('.pdf', '.html')

with open(md_file, 'r', encoding='utf-8') as f:
    md_content = f.read()

html_body = markdown.markdown(md_content, extensions=['tables', 'fenced_code'])

html = f"""<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
@page {{
    size: A4;
    margin: 2cm 2.5cm;
}}
body {{
    font-family: "Noto Sans CJK SC", "Source Han Sans SC", "Microsoft YaHei", "PingFang SC", sans-serif;
    font-size: 11pt;
    line-height: 1.7;
    color: #333;
}}
h1 {{
    font-size: 22pt;
    color: #1a1a2e;
    border-bottom: 3px solid #0f3460;
    padding-bottom: 8px;
    margin-top: 0;
}}
h2 {{
    font-size: 16pt;
    color: #16213e;
    border-bottom: 1px solid #ccc;
    padding-bottom: 5px;
    margin-top: 30px;
    page-break-after: avoid;
}}
h3 {{
    font-size: 13pt;
    color: #0f3460;
    margin-top: 20px;
    page-break-after: avoid;
}}
h4 {{
    font-size: 11.5pt;
    color: #333;
    margin-top: 15px;
    page-break-after: avoid;
}}
table {{
    width: 100%;
    border-collapse: collapse;
    margin: 10px 0;
    font-size: 10pt;
}}
th {{
    background-color: #0f3460;
    color: white;
    padding: 8px 10px;
    text-align: left;
    font-weight: 600;
}}
td {{
    padding: 7px 10px;
    border-bottom: 1px solid #e0e0e0;
}}
tr:nth-child(even) {{
    background-color: #f8f9fa;
}}
img {{
    max-width: 75%;
    max-height: 300px;
    width: auto;
    height: auto;
    margin: 4px 0;
    border: 1px solid #e0e0e0;
    border-radius: 4px;
    display: block;
}}
hr {{
    border: none;
    border-top: 1px solid #ddd;
    margin: 25px 0;
}}
p {{
    margin: 6px 0;
    orphans: 3;
    widows: 3;
}}
ul, ol {{
    margin: 6px 0;
    padding-left: 20px;
}}
li {{
    margin: 3px 0;
}}
</style>
</head>
<body>
{html_body}
</body>
</html>"""

# Make image paths absolute so HTML in pdf/ dir can find images in parent dir
import re
def abs_img_paths(html_str, img_base):
    def repl(m):
        src = m.group(1)
        if src.startswith(('http', 'data:', 'file://')):
            return m.group(0)
        abs_p = os.path.abspath(os.path.join(img_base, src))
        return m.group(0).replace(src, f'file://{abs_p}')
    return re.sub(r'<img[^>]+src="([^"]+)"', repl, html_str)

html = abs_img_paths(html, str(base_dir))

# Write HTML file
with open(html_file, 'w', encoding='utf-8') as f:
    f.write(html)

print(f"HTML written: {html_file}")

# Use Playwright to render to PDF
from playwright.sync_api import sync_playwright

with sync_playwright() as p:
    browser = p.chromium.launch(args=['--no-sandbox', '--disable-setuid-sandbox'])
    page = browser.new_page()
    page.goto(f'file://{Path(html_file).resolve()}', wait_until='networkidle')
    page.pdf(
        path=pdf_file,
        format='A4',
        margin={'top': '2cm', 'bottom': '2cm', 'left': '2.5cm', 'right': '2.5cm'},
        print_background=True,
    )
    browser.close()

size_mb = os.path.getsize(pdf_file) / (1024 * 1024)
print(f"PDF generated: {pdf_file} ({size_mb:.1f} MB)")

# Clean up HTML
os.remove(html_file)
print("Temp HTML cleaned up.")
