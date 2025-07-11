//*****************************************************************************
// Copyright 2025 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//*****************************************************************************

import styles from './index.module.scss';
import favicon from '@/assets/favicon.png';

export default function TopHeader() {
  return (
    <div className={styles.topHeader}>
      <div className={styles.headerLeft}>
        <div className={styles.project}>
          <img
            src={favicon}
            alt=""
          />
          <div>OADIN</div>
        </div>
      </div>
      <div className={styles.personIcon}>
        <img
          src={favicon}
          alt=""
        />
      </div>
    </div>
  );
}
