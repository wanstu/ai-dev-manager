// Run the real-browser Gateway access policy scenario across supported viewports.
process.env.ADM_UI_SMOKE_FOCUS = 'gateway-access';
require('./browser-smoke.cjs');
