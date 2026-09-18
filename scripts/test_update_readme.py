import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from update_readme import update_xkcd, update_calvin_and_hobbes


SAMPLE_README = """# Hello
<table>
  <tr>
    <td>
      <!-- START_XKCD_IMG -->
      <img src="old_xkcd.png" alt="old"/>
      <!-- END_XKCD_IMG -->
    </td>
  </tr>
  <tr>
    <td>
      <!-- START_XKCD_ALT -->
      <sub>old alt</sub>
      <!-- END_XKCD_ALT -->
    </td>
  </tr>
</table>

<!-- START_CALVIN_AND_HOBBES_SECTION -->
<table border="0" cellspacing="0" cellpadding="0" style="border-collapse: collapse;">
  <tr>
    <td align="center"><h3 style="margin: 0;">Daily Calvin and Hobbes Comic</h3></td>
  </tr>
  <tr>
    <td align="center">
      <img src="old_ch.png" alt="Calvin and Hobbes Comic"/>
    </td>
  </tr>
</table>
<!-- END_CALVIN_AND_HOBBES_SECTION -->
"""


class TestUpdateReadme(unittest.TestCase):
    def test_update_xkcd(self):
        new_url = "https://imgs.xkcd.com/comics/test.png"
        new_alt = "A joke with quotes \"like this\" & symbols <tag> \\backslashes\\."
        result = update_xkcd(SAMPLE_README, new_url, new_alt)

        self.assertIn(f'<img src="{new_url}"', result)
        self.assertIn('&quot;like this&quot;', result)
        self.assertIn('&lt;tag&gt;', result)
        self.assertIn('\\backslashes\\', result)
        self.assertNotIn("old_xkcd.png", result)

    def test_update_calvin_and_hobbes(self):
        new_url = "https://featureassets.gocomics.com/assets/new_comic.png"
        result = update_calvin_and_hobbes(SAMPLE_README, new_url)

        self.assertIn(f'<img src="{new_url}"', result)
        self.assertNotIn("old_ch.png", result)

    def test_empty_urls_do_not_modify(self):
        self.assertEqual(update_xkcd(SAMPLE_README, "", "alt"), SAMPLE_README)
        self.assertEqual(update_calvin_and_hobbes(SAMPLE_README, ""), SAMPLE_README)


if __name__ == "__main__":
    unittest.main()
