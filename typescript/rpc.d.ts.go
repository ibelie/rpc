// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package typescript

const RPC_D_TS = `// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

declare module ibelie.rpc {
	class Entity {
		isAwake: boolean;
		Awake(): void;
		Sleep(): void;
	}

	class Component {
		Entity: Entity;
		constructor(entity: Entity);
	}
}
`
