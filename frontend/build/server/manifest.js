const manifest = (() => {
function __memo(fn) {
	let value;
	return () => value ??= (value = fn());
}

return {
	appDir: "_app",
	appPath: "_app",
	assets: new Set([]),
	mimeTypes: {},
	_: {
		client: {start:"_app/immutable/entry/start.xOCZY_hK.js",app:"_app/immutable/entry/app.DhWrKlXM.js",imports:["_app/immutable/entry/start.xOCZY_hK.js","_app/immutable/chunks/DQcoMPka.js","_app/immutable/chunks/DZJbeXPm.js","_app/immutable/chunks/CKcRsUGc.js","_app/immutable/chunks/BdGAytfj.js","_app/immutable/entry/app.DhWrKlXM.js","_app/immutable/chunks/DZJbeXPm.js","_app/immutable/chunks/DRMUDMiI.js","_app/immutable/chunks/D1vpSegM.js","_app/immutable/chunks/CPI-Sj6W.js","_app/immutable/chunks/BdGAytfj.js","_app/immutable/chunks/LXw8YH9d.js","_app/immutable/chunks/CWTv1oOY.js","_app/immutable/chunks/CKcRsUGc.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
		nodes: [
			__memo(() => import('./chunks/0-CQ6X3d9U.js')),
			__memo(() => import('./chunks/1-DOfZYss2.js')),
			__memo(() => import('./chunks/2-CUv0E03b.js')),
			__memo(() => import('./chunks/3-DAQCN8tq.js')),
			__memo(() => import('./chunks/4-CUEFy1ro.js')),
			__memo(() => import('./chunks/5-BmWioCle.js')),
			__memo(() => import('./chunks/6-CT9wwPDV.js')),
			__memo(() => import('./chunks/7-C5Qpcl6E.js')),
			__memo(() => import('./chunks/8-fMeaiz6L.js')),
			__memo(() => import('./chunks/9-ZTqoyY41.js')),
			__memo(() => import('./chunks/10-Dm5AqcHI.js'))
		],
		remotes: {
			
		},
		routes: [
			{
				id: "/",
				pattern: /^\/$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 2 },
				endpoint: null
			},
			{
				id: "/accounts",
				pattern: /^\/accounts\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 3 },
				endpoint: null
			},
			{
				id: "/contacts",
				pattern: /^\/contacts\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 4 },
				endpoint: null
			},
			{
				id: "/dashboard",
				pattern: /^\/dashboard\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 5 },
				endpoint: null
			},
			{
				id: "/invoices",
				pattern: /^\/invoices\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 6 },
				endpoint: null
			},
			{
				id: "/journal",
				pattern: /^\/journal\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 7 },
				endpoint: null
			},
			{
				id: "/login",
				pattern: /^\/login\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 8 },
				endpoint: null
			},
			{
				id: "/payments",
				pattern: /^\/payments\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 9 },
				endpoint: null
			},
			{
				id: "/reports",
				pattern: /^\/reports\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 10 },
				endpoint: null
			}
		],
		prerendered_routes: new Set([]),
		matchers: async () => {
			
			return {  };
		},
		server_assets: {}
	}
}
})();

const prerendered = new Set([]);

const base = "";

export { base, manifest, prerendered };
//# sourceMappingURL=manifest.js.map
