export async function selectSettingsCategory(page, category) {
  await page.locator('.settings-workspace').waitFor({state:'visible'})
  const select = page.locator('.settings-category-select select')
  if (await select.isVisible()) {
    await select.selectOption(category)
  } else {
    await page.locator(`#settings-tab-${category}`).click()
  }
}
