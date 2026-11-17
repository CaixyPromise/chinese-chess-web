import { test, expect } from '@playwright/test';

test('real PVE: legal targets, C++ reply, keyboard, undo, reconnect, refresh and resignation', async ({page,context}) => {
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto('/');await page.getByRole('button',{name:'开始新对局'}).click();await page.waitForURL('**/games/*');
 await expect(page.getByRole('gridcell',{name:'红马 b0',exact:true})).toHaveAttribute('aria-disabled','false');
 await page.getByRole('gridcell',{name:'红马 b0',exact:true}).click();await expect(page.getByRole('gridcell',{name:'空位 c2',exact:true})).toHaveAttribute('data-target','true');
 await page.screenshot({path:'../build/e2e/screenshots/desktop-selected.png',fullPage:true});
 const moves=page.waitForResponse(r=>r.url().endsWith('/commands')&&r.request().method()==='POST');await page.getByRole('gridcell',{name:'空位 c2',exact:true}).click();expect((await (await moves).json()).moves).toHaveLength(1);
 await expect(page.getByRole('status')).toContainText('轮到你了');
 let view=await (await page.request.get('/api'+new URL(page.url()).pathname)).json();expect(view.moves).toHaveLength(2);expect(view.moves[0]).toBe('b0c2');
 await page.getByRole('gridcell',{name:'红马 c2',exact:true}).focus();await page.keyboard.press('ArrowRight');await expect(page.locator('[data-square=d2]')).toBeFocused();
 await page.reload();await expect(page.getByRole('status')).toContainText('轮到你了');await expect(page.getByRole('gridcell',{name:'红马 c2',exact:true})).toBeVisible();
 await context.setOffline(true);await expect(page.getByRole('status')).toContainText('重新连接');await context.setOffline(false);await expect(page.getByRole('status')).toContainText('轮到你了',{timeout:15000});
 await page.getByRole('button',{name:'悔棋',exact:false}).click();await expect(page.getByRole('gridcell',{name:'红马 b0',exact:true})).toHaveAttribute('aria-disabled','false');
 view=await (await page.request.get('/api'+new URL(page.url()).pathname)).json();expect(view.moves).toHaveLength(0);
 await page.getByRole('button',{name:'认输',exact:false}).click();await expect(page.getByRole('button',{name:'继续下棋'})).toBeFocused();await page.getByRole('dialog').getByRole('button',{name:/认\s*输/}).click();await expect(page.getByRole('status')).toContainText('本局结束');await expect(page.getByRole('dialog')).toHaveCount(0);await page.screenshot({path:'../build/e2e/screenshots/result.png',fullPage:true});expect(errors).toEqual([]);
});

test('real black-side game and mobile layouts, flipped coordinates, history and export',async({page})=>{
 await page.setViewportSize({width:390,height:844});await page.goto('/');await page.getByText('执黑 · 后手',{exact:true}).click();await expect(page.getByRole('radio',{name:'执黑 · 后手'})).toBeChecked();await page.getByRole('button',{name:'开始新对局'}).click();await page.waitForURL('**/games/*');await expect(page.getByRole('status')).toContainText('轮到你了');
 const view=await(await page.request.get('/api'+new URL(page.url()).pathname)).json();expect(view.human).toBe('black');expect(view.moves).toHaveLength(1);
 const before=await page.locator('[data-square=a9]').boundingBox();await page.getByRole('button',{name:'翻转',exact:false}).click();const after=await page.locator('[data-square=a9]').boundingBox();expect(before!.x).toBeGreaterThan(after!.x);
 for (const width of [390,320]){await page.setViewportSize({width,height:844});await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth)).toBe(width);await page.screenshot({path:`../build/e2e/screenshots/mobile-${width}.png`,fullPage:true});}
 await page.getByRole('button',{name:'棋谱',exact:false}).click();await expect(page.getByRole('dialog')).toBeVisible();await expect.poll(()=>page.getByRole('dialog').evaluate(el=>{const r=el.getBoundingClientRect();return r.left>=0&&r.right<=innerWidth+1})).toBe(true);expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBe(320);await page.screenshot({path:'../build/e2e/screenshots/mobile-history.png'});
 const download=page.waitForEvent('download');await page.getByRole('button',{name:'导出',exact:false}).click();expect((await download).suggestedFilename()).toMatch(/象棋棋谱.*json/);await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(0);
});

test('controlled error presentation preserves form and offers recovery (HTTP fault injection)',async({page})=>{
 await page.goto('/');await expect(page.getByRole('button',{name:'开始新对局'})).toBeEnabled();
 await page.route('**/api/games',route=>route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({code:'INTERNAL_STACK_SECRET'})}));
 await page.getByRole('button',{name:'开始新对局'}).click();await expect(page.getByRole('alert')).toContainText('操作暂时未能完成');await expect(page.locator('body')).not.toContainText('INTERNAL_STACK_SECRET');await page.screenshot({path:'../build/e2e/screenshots/error.png',fullPage:true});
 await page.unroute('**/api/games');await page.getByRole('button',{name:'开始新对局'}).click();await page.waitForURL('**/games/*');await expect(page.getByRole('status')).toContainText('轮到你了');
});
