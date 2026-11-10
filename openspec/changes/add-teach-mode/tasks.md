## 1. Data model

- [ ] 1.1 Add a migration for ordered guide steps per user and game with title, body, minutes and a unique position
- [ ] 1.2 Add `last_teach_minutes` to the guide record
- [ ] 1.3 Keep existing guides valid with zero steps

## 2. Authoring

- [ ] 2.1 Extend the guide editor with add, edit, reorder and remove for steps
- [ ] 2.2 Persist reordering inside one `db.WithTx` and recompute the total
- [ ] 2.3 Add a "split into steps" action that seeds steps from the existing free-text guide
- [ ] 2.4 Show the total teach time in the editor

## 3. Teach view

- [ ] 3.1 Add `/guide/{gameID}/teach` with one section per step and inline photos
- [ ] 3.2 Build the timer island with `sessionStorage` persistence, reduced-motion respect and coarse `aria-live` announcements
- [ ] 3.3 Provide a no-JavaScript fallback that keeps every step readable
- [ ] 3.4 Record elapsed minutes on completion and prefer them over inference afterwards
- [ ] 3.5 Offer "start teach" from the game detail page and from a planner result

## 4. Print and QR

- [ ] 4.1 Add a print stylesheet dropping navigation and interactive controls, black on white
- [ ] 4.2 Ensure steps do not split across pages and photos stay legible or are omitted
- [ ] 4.3 Generate a per-game QR code server-side from the base URL with no token in the payload
- [ ] 4.4 Add the printed caption and sign-in instruction under the code

## 5. Verification

- [ ] 5.1 Test that the teach route requires authentication and discloses nothing when signed out
- [ ] 5.2 Test step ordering persistence and that a failed reorder changes nothing
- [ ] 5.3 Test the elapsed-time formatter in Vitest
- [ ] 5.4 Test print output with Playwright `emulateMedia('print')`
- [ ] 5.5 Run axe checks on the teach and print views and confirm keyboard operation of the timer
