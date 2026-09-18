import html
import os
import re
import sys


def update_xkcd(content: str, img_url: str, alt_text: str) -> str:
    """Updates the XKCD image URL and alt text in the README content."""
    if not img_url:
        return content

    escaped_alt_attr = html.escape(alt_text, quote=True)
    escaped_alt_text = html.escape(alt_text, quote=False)

    content = re.sub(
        r"(<!-- START_XKCD_IMG -->)(.*?)(<!-- END_XKCD_IMG -->)",
        lambda m: f'{m.group(1)}\n            <a href="https://xkcd.com/" target="_blank"><img src="{img_url}" alt="{escaped_alt_attr}"/></a>\n            {m.group(3)}',
        content,
        flags=re.DOTALL,
    )
    content = re.sub(
        r"(<!-- START_XKCD_ALT -->)(.*?)(<!-- END_XKCD_ALT -->)",
        lambda m: f"{m.group(1)}\n            <sub>{escaped_alt_text}</sub>\n            {m.group(3)}",
        content,
        flags=re.DOTALL,
    )
    return content


def update_calvin_and_hobbes(content: str, ch_url: str) -> str:
    """Updates or adds the Calvin and Hobbes comic section in the README content."""
    if not ch_url:
        return content

    ch_section_start_tag = "<!-- START_CALVIN_AND_HOBBES_SECTION -->"
    ch_section_end_tag = "<!-- END_CALVIN_AND_HOBBES_SECTION -->"

    ch_html_content = f"""<table border="0" cellspacing="0" cellpadding="0" style="border-collapse: collapse;">
  <tr>
    <td align="center"><h3 style="margin: 0;">Daily Calvin and Hobbes Comic</h3></td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://www.gocomics.com/calvinandhobbes" target="_blank">
        <img src="{ch_url}" alt="Calvin and Hobbes Comic"/>
      </a>
    </td>
  </tr>
</table>"""

    ch_section_pattern = re.compile(
        rf"({re.escape(ch_section_start_tag)})(.*?)({re.escape(ch_section_end_tag)})",
        re.DOTALL,
    )

    if ch_section_pattern.search(content):
        content = ch_section_pattern.sub(
            lambda m: f"{m.group(1)}\n{ch_html_content}\n{m.group(3)}",
            content,
            count=1,
        )
    else:
        xkcd_table_pattern = re.compile(
            r"(<table[^>]*>.*?<!-- START_XKCD_IMG -->.*?<!-- END_XKCD_ALT -->.*?</table>)",
            re.DOTALL,
        )
        xkcd_match = xkcd_table_pattern.search(content)
        if xkcd_match:
            end_pos = xkcd_match.end()
            full_section = f"\n\n{ch_section_start_tag}\n{ch_html_content}\n{ch_section_end_tag}"
            content = content[:end_pos] + full_section + content[end_pos:]
        else:
            print("::warning::Could not find XKCD table to reliably insert Calvin and Hobbes section after it.")

    return content


def main() -> None:
    readme_path = os.getenv("README_PATH", "README.md")

    xkcd_img_url = os.getenv("XKCD_IMG_URL", "")
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
        sys.exit(1)

    if xkcd_outcome == "success" and xkcd_img_url:
        print("Attempting to update XKCD section.")
        content = update_xkcd(content, xkcd_img_url, xkcd_alt_text)
    else:
        print(f"Skipping XKCD update (Outcome: {xkcd_outcome}, URL empty: {not xkcd_img_url}).")

    content_after_xkcd = content

    if ch_outcome == "success" and ch_url:
        print("Attempting to update/add Calvin and Hobbes section.")
        content = update_calvin_and_hobbes(content, ch_url)
    else:
        print(f"Skipping Calvin and Hobbes update (Outcome: {ch_outcome}, URL empty: {not ch_url}).")

    if content != original_content:
        try:
            with open(readme_path, "w", encoding="utf-8") as f:
                f.write(content)
            print(f"README.md successfully updated at {readme_path}")
            if content_after_xkcd != original_content and content_after_xkcd != content:
                print("Both XKCD and Calvin and Hobbes sections were modified.")
            elif content_after_xkcd != original_content:
                print("Only XKCD section was modified.")
            else:
                print("Only Calvin and Hobbes section was modified.")
        except Exception as e:
            print(f"::error::Failed to write updated README.md: {e}")
            sys.exit(1)
    else:
        print("README.md content unchanged after processing.")


if __name__ == "__main__":
    main()