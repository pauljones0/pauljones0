import os
import re

readme_path = "README.md"

xkcd_img_url = os.getenv("XKCD_IMG_URL", "")
xkcd_img_url = xkcd_img_url.replace("\\\\", "")
xkcd_alt_text = os.getenv("XKCD_ALT_TEXT", "XKCD comic")
xkcd_outcome = os.getenv("XKCD_FETCH_OUTCOME", "failure")

ch_url = os.getenv("CH_URL", "")
ch_outcome = os.getenv("CH_FETCH_OUTCOME", "failure")

try:
    with open(readme_path, "r", encoding="utf-8") as f:
        content = f.read()
    original_content = content
except FileNotFoundError:
    print(f"::error::README.md not found at {readme_path}")
    exit(1)

# XKCD Update
if xkcd_outcome == "success" and xkcd_img_url:
    print("Attempting to update XKCD section.")
    # Ensure alt text is properly escaped for HTML attribute
    escaped_xkcd_alt_text = xkcd_alt_text.replace('"', '&quot;') # Keep this for attribute value
    content = re.sub(
        r"(<!-- START_XKCD_IMG -->)(.*?)(<!-- END_XKCD_IMG -->)",
        rf"\1\n            <img src=\"{xkcd_img_url}\" alt=\"{escaped_xkcd_alt_text}\"/>\n            \3",
        content,
        flags=re.DOTALL,
    )
    content = re.sub(
        r"(<!-- START_XKCD_ALT -->)(.*?)(<!-- END_XKCD_ALT -->)",
        rf"\1\n            <sub>{xkcd_alt_text}</sub>\n            \3", # Alt text here is content, not attribute
        content,
        flags=re.DOTALL,
    )
else:
    print(f"Skipping XKCD update (Outcome: {xkcd_outcome}, URL empty: {not xkcd_img_url}).")

content_after_xkcd_update = content

# Calvin and Hobbes Update
ch_section_start_tag = "<!-- START_CALVIN_AND_HOBBES_SECTION -->"
ch_section_end_tag = "<!-- END_CALVIN_AND_HOBBES_SECTION -->"

if ch_outcome == "success" and ch_url:
    print("Attempting to update/add Calvin and Hobbes section.")
    ch_html_content = f"""<table border="0" cellspacing="0" cellpadding="0" style="border-collapse: collapse;">
  <tr>
    <td align="center"><h3 style="margin: 0;">Daily Calvin and Hobbes Comic</h3></td>
  </tr>
  <tr>
    <td align="center">
      <img src="{ch_url}" alt="Calvin and Hobbes Comic"/>
    </td>
  </tr>
</table>"""

    ch_section_pattern = re.compile(rf"({re.escape(ch_section_start_tag)})(.*?)({re.escape(ch_section_end_tag)})", re.DOTALL)

    if ch_section_pattern.search(content):
        content = ch_section_pattern.sub(
            lambda m: f"{m.group(1)}\n{ch_html_content}\n{m.group(3)}",
            content,
            count=1
        )
        print("Calvin and Hobbes section updated.")
    else:
        print("Calvin and Hobbes section markers not found. Attempting to add new section after XKCD table.")
        xkcd_table_pattern = re.compile(r'(<table[^>]*>.*?<!-- START_XKCD_IMG -->.*?<!-- END_XKCD_ALT -->.*?</table>)', re.DOTALL)
        xkcd_match = xkcd_table_pattern.search(content)

        if xkcd_match:
            end_of_xkcd_table_pos = xkcd_match.end()
            ch_full_section_with_markers = f"\n\n{ch_section_start_tag}\n{ch_html_content}\n{ch_section_end_tag}"
            content = content[:end_of_xkcd_table_pos] + ch_full_section_with_markers + content[end_of_xkcd_table_pos:]
            print("Calvin and Hobbes section added after XKCD table.")
        else:
            print("::warning::Could not find XKCD table to reliably insert Calvin and Hobbes section after it. C&H section not added.")
else:
    print(f"Skipping Calvin and Hobbes update (Outcome: {ch_outcome}, URL empty: {not ch_url}). Section will remain unchanged if it exists.")

if content != original_content:
    try:
        with open(readme_path, "w", encoding="utf-8") as f:
            f.write(content)
        print(f"README.md successfully updated at {readme_path}")
        if content_after_xkcd_update != original_content and content_after_xkcd_update != content:
            print("Both XKCD and Calvin and Hobbes sections were modified (or C&H was added).")
        elif content_after_xkcd_update != original_content:
            print("Only XKCD section was modified.")
        elif content != original_content :
            print("Only Calvin and Hobbes section was modified or added.")
    except Exception as e:
        print(f"::error::Failed to write updated README.md: {e}")
        exit(1)
else:
    print("README.md content unchanged after processing.")

print("\nVerifying README.md content (first ~20 lines after potential update):")
try:
  with open(readme_path, "r", encoding="utf-8") as f:
      for i, line in enumerate(f):
          if i < 20:
              print(line.strip())
          else:
              break
except Exception as e:
  print(f"::warning::Could not re-read README for verification: {e}")